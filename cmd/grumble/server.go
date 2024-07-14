// Copyright (c) 2010-2011 The Grumble Authors
// The use of this source code is goverened by a BSD-style
// license that can be found in the LICENSE-file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha512"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"google.golang.org/protobuf/proto"
	"mumble.info/grumble/pkg/acl"
	"mumble.info/grumble/pkg/ban"
	"mumble.info/grumble/pkg/database"
	"mumble.info/grumble/pkg/htmlfilter"
	"mumble.info/grumble/pkg/logtarget"
	"mumble.info/grumble/pkg/mumbleproto"
	"mumble.info/grumble/pkg/serverconf"
	"mumble.info/grumble/pkg/sessionpool"
	"mumble.info/grumble/pkg/web"
)

// The default port a Murmur server listens on
const DefaultPort = 64738
const DefaultWebPort = 443
const UDPPacketSize = 1024

const LogOpsBeforeSync = 100
const CeltCompatBitstream = -2147483637
const (
	StateClientConnected = iota
	StateServerSentVersion
	StateClientSentVersion
	StateClientAuthenticated
	StateClientReady
	StateClientDead
)

type KeyValuePair struct {
	Key   string
	Value string
	Reset bool
}

// A Murmur server instance
type Server struct {
	Id int64

	tcpl      *net.TCPListener
	tlsl      net.Listener
	udpconn   *net.UDPConn
	tlscfg    *tls.Config
	webwsl    *web.Listener
	webtlscfg *tls.Config
	webhttp   *http.Server
	bye       chan bool
	netwg     sync.WaitGroup
	running   bool

	incoming       chan *Message
	voicebroadcast chan *VoiceBroadcast
	tempRemove     chan *Channel

	// Signals to the server that a client has been successfully
	// authenticated.
	clientAuthenticated chan *Client

	// Server configuration
	cfg serverconf.ConfigRepo

	// Clients
	clients map[uint32]*Client

	// Host, host/port -> client mapping
	hmutex    sync.Mutex
	hclients  map[string][]*Client
	hpclients map[string]*Client

	// Codec information
	AlphaCodec       int32
	BetaCodec        int32
	PreferAlphaCodec bool
	Opus             bool

	// Database
	DB *database.GrumbleDb

	// Channels
	Channels       map[int]*Channel
	nextTempChanID int

	// Users
	Users map[uint32]*User

	// Sessions
	pool *sessionpool.SessionPool

	// Bans
	banlock sync.RWMutex
	Bans    []ban.Ban

	// Logging
	*log.Logger

	listenerManager ChannelListenerManager

	// Other configurations
	DataDir string
	Version ClientVersion
}

type clientLogForwarder struct {
	client *Client
	logger *log.Logger
}

func (lf clientLogForwarder) Write(incoming []byte) (int, error) {
	buf := new(bytes.Buffer)
	buf.WriteString(fmt.Sprintf("<%v:%v(%v)> ", lf.client.Session(), lf.client.ShownName(), lf.client.UserId()))
	buf.Write(incoming)
	lf.logger.Output(3, buf.String())
	return len(incoming), nil
}

// Allocate a new Murmur instance
func NewServer(id int64, dataDir string, db *database.GrumbleDb) (s *Server, err error) {
	s = new(Server)

	s.Id = id
	s.DB = db

	s.cfg = NewConfigWrapper(db, uint64(id))
	s.DataDir = dataDir
	s.Version = VersionFromComponent(1, 5, 0)
	s.nextTempChanID = 10000000 // use a high channel id for identity

	s.Channels = make(map[int]*Channel)
	s.Users = make(map[uint32]*User)
	s.Logger = log.New(logtarget.Default, fmt.Sprintf("[%v] ", s.Id), log.LstdFlags|log.Lmicroseconds)

	// Generate random password for superuser
	randomBytes := make([]byte, 9)
	rand.Read(randomBytes)
	rootPassword := base64.StdEncoding.EncodeToString(randomBytes)

	// Create root account, channel, groups
	db.Transaction(func(tx *database.DbTx) error {
		isNew, err := tx.ServerInit(uint64(id), s.generatePasswordHash(rootPassword))
		if err != nil {
			return err
		}

		if isNew {
			s.Logger.Printf("Password for 'SuperUser' set to '%s'", rootPassword)
		}

		// If new server, create initial ACL table
		if isNew {
			err = tx.ACLAdd(database.NewACL(uint64(id), 0, 1).WithGroup("admin").Apply(true, true).Grant(acl.WritePermission))
			if err != nil {
				return err
			}
			err = tx.ACLAdd(database.NewACL(uint64(id), 0, 2).WithGroup("auth").Apply(true, true).Grant(acl.TempChannelPermission))
			if err != nil {
				return err
			}
			err = tx.ACLAdd(database.NewACL(uint64(id), 0, 3).WithGroup("all").Apply(true, false).Grant(acl.SelfRegisterPermission))
			if err != nil {
				return err
			}
		}

		// Load channel data from db
		channels, err := tx.ChannelList(uint64(id))
		if err != nil {
			return err
		}
		for _, v := range channels {
			channel := NewChannel(int(v.ChannelID), v.Name)
			channel.ACL.InheritACL = v.InheritACL

			// Additional information in channel_info table
			info, err := tx.ChannelInfoGets(v.ServerID, v.ChannelID, []database.ChannelInfoKey{
				database.ChannelDescription,
				database.ChannelMaxUsers,
				database.ChannelPosition,
			})
			if err != nil {
				return err
			}
			if v, ok := info[database.ChannelDescription]; ok {
				channel.DescriptionBlob = v
			}
			if v, ok := info[database.ChannelMaxUsers]; ok {
				channel.MaxUsers, _ = strconv.Atoi(v)
			}
			if v, ok := info[database.ChannelPosition]; ok {
				channel.Position, _ = strconv.Atoi(v)
			}

			// Restore ACL data
			acls, err := tx.ACLGet(v.ServerID, v.ChannelID)
			if err != nil {
				return err
			}
			for _, a := range acls {
				ACL := acl.ACL{
					UserId:    int(a.UserID.Int64),
					Group:     a.GroupName,
					ApplyHere: a.ApplyHere,
					ApplySubs: a.ApplySub,
					Allow:     acl.Permission(a.GrantPriv),
					Deny:      acl.Permission(a.RevokePriv),
				}
				if !a.UserID.Valid {
					ACL.UserId = -1
				}
				channel.ACL.ACLs = append(channel.ACL.ACLs, ACL)
			}

			// ACL group data
			groups, err := tx.GroupGetByChannel(v.ServerID, v.ChannelID)
			if err != nil {
				return err
			}
			for _, g := range groups {
				group := acl.Group{
					Name:        g.Name,
					Inherit:     g.Inherit,
					Inheritable: g.Inheritable,
					Add:         make(map[int]bool),
					Remove:      make(map[int]bool),
					Temporary:   make(map[int]bool),
				}

				members, err := tx.GroupMemberList(v.ServerID, g.GroupID)
				if err != nil {
					return err
				}
				for _, m := range members {
					if m.Addit {
						group.Add[int(m.UserID)] = true
					} else {
						group.Remove[int(m.UserID)] = true
					}
				}

				channel.ACL.Groups[g.Name] = group
			}
			s.Channels[int(v.ChannelID)] = channel
		}
		for _, v := range channels {
			if v.ParentID.Valid {
				s.Channels[int(v.ParentID.Int64)].AddChild(s.Channels[int(v.ChannelID)])
			}
		}

		return nil
	})

	s.listenerManager = NewChannelListenerManager()
	// Ban list
	s.readBanList()

	return
}

// Debugf implements debug-level printing for Servers.
func (server *Server) Debugf(format string, v ...interface{}) {
	server.Printf(format, v...)
}

// RootChannel gets a pointer to the root channel
func (server *Server) RootChannel() *Channel {
	root, exists := server.Channels[0]
	if !exists {
		server.Fatalf("Not Root channel found for server")
	}
	return root
}

func (server *Server) generatePasswordHash(password string) database.UserPasswordHash {
	saltBytes := make([]byte, 8)
	_, err := rand.Read(saltBytes)
	if err != nil {
		server.Fatalf("Unable to read from crypto/rand: %v", err)
	}

	const PBKDF2Iter = 4096
	key := pbkdf2.Key([]byte(password), saltBytes, PBKDF2Iter, 48, sha512.New384)
	return database.UserPasswordHash{
		Salt:          []byte(hex.EncodeToString(saltBytes)),
		Hash:          hex.EncodeToString(key),
		KDFIterations: PBKDF2Iter,
	}
}

func (server *Server) verifyPasswordHash(hash database.UserPasswordHash, password string) (bool, error) {
	saltBytes, err := hex.DecodeString(string(hash.Salt))
	if err != nil {
		return false, err
	}

	key := pbkdf2.Key([]byte(password), saltBytes, hash.KDFIterations, 48, sha512.New384)
	return strings.EqualFold(hex.EncodeToString(key), strings.ToLower(hash.Hash)), nil
}

// SetSuperUserPassword sets password as the new SuperUser password
func (server *Server) SetSuperUserPassword(password string) {
	tx := server.DB.Tx()
	err := tx.UserSetAuth(uint64(server.Id), 0, server.generatePasswordHash(password))
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to change SuperUser password: %v", err)
	}
	tx.Commit()
}

// SetServerPassword sets password as the new Server password
func (server *Server) SetServerPassword(password string) {
	tx := server.DB.Tx()
	err := tx.ConfigSet(uint64(server.Id), "password", password)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to change server password: %v", err)
	}
	tx.Commit()
}

// CheckSuperUserPassword checks whether password matches the set SuperUser password.
func (server *Server) CheckSuperUserPassword(password string) bool {
	tx := server.DB.Tx()
	user, err := tx.UserGetAuth(uint64(server.Id), "SuperUser")
	if err != nil {
		tx.Rollback()
		server.Fatalf("Unable to get superuser auth info: %v", err)
	}
	tx.Commit()

	ok, err := server.verifyPasswordHash(user.Password, password)
	if err != nil {
		server.Fatalf("Unable to verify password: %v", err)
	}
	return ok
}

// CheckServerPassword checks whether password matches the set Server password.
func (server *Server) CheckServerPassword(password string) bool {
	tx := server.DB.Tx()
	pw, err := tx.ConfigGet(uint64(server.Id), "password")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to get server password: %v", err)
	}
	tx.Commit()
	return pw == password
}

func (server *Server) hasServerPassword() bool {
	tx := server.DB.Tx()
	pw, err := tx.ConfigGet(uint64(server.Id), "password")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to get server password: %v", err)
	}
	tx.Commit()

	return pw != ""
}

// Called by the server to initiate a new client connection.
func (server *Server) handleIncomingClient(conn net.Conn) (err error) {
	client := new(Client)
	addr := conn.RemoteAddr()
	if addr == nil {
		err = errors.New("unable to extract address for client")
		return
	}

	client.lf = &clientLogForwarder{client, server.Logger}
	client.Logger = log.New(client.lf, "", 0)

	client.session = server.pool.Get()
	client.Printf("New connection: %v (%v)", conn.RemoteAddr(), client.Session())

	client.tcpaddr = addr.(*net.TCPAddr)
	client.server = server
	client.conn = conn
	client.reader = bufio.NewReader(client.conn)

	client.state = StateClientConnected

	client.udprecv = make(chan []byte)
	client.voiceTargets = make(map[uint32]*VoiceTarget)

	client.user = nil
	client.Bandwidth = NewBandwidthRecorder()
	client.GlobalLimit = NewRateLimit(uint(server.cfg.IntValue("MessageLimit")), uint(server.cfg.IntValue("MessageBurst")))
	client.PluginLimit = NewRateLimit(uint(server.cfg.IntValue("PluginMessageLimit")), uint(server.cfg.IntValue("PluginMessageBurst")))

	// Extract user's cert hash
	// Only consider client certificates for direct connections, not WebSocket connections.
	// We do not support TLS-level client certificates for WebSocket client.
	if tlsconn, ok := client.conn.(*tls.Conn); ok {
		err = tlsconn.Handshake()
		if err != nil {
			client.Printf("TLS handshake failed: %v", err)
			client.Disconnect()
			return
		}

		state := tlsconn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			hash := sha1.New()
			hash.Write(cert.Raw)
			sum := hash.Sum(nil)
			// todo(jim-k): verify certificate
			client.certHash = hex.EncodeToString(sum)
			if len(cert.EmailAddresses) > 0 {
				client.Email = cert.EmailAddresses[0]
			}
		}

		// Check whether the client's cert hash is banned
		if server.IsCertHashBanned(client.CertHash()) {
			client.Printf("Certificate hash is banned")
			client.Disconnect()
			return
		}
	}

	// Launch network readers
	go client.tlsRecvLoop()
	go client.udpRecvLoop()

	return
}

// RemoveClient removes a disconnected client from the server's
// internal representation.
func (server *Server) RemoveClient(client *Client, kicked bool) {
	server.hmutex.Lock()
	host := client.tcpaddr.IP.String()
	oldclients := server.hclients[host]
	newclients := []*Client{}
	for _, hostclient := range oldclients {
		if hostclient != client {
			newclients = append(newclients, hostclient)
		}
	}
	server.hclients[host] = newclients
	if client.udpaddr != nil {
		delete(server.hpclients, client.udpaddr.String())
	}
	server.hmutex.Unlock()

	delete(server.clients, client.Session())
	server.pool.Reclaim(client.Session())

	// Remove client from channel
	channel := client.Channel
	if channel != nil {
		channel.RemoveClient(client)
	}

	// If the user was not kicked, broadcast a UserRemove message.
	// If the user is disconnect via a kick, the UserRemove message has already been sent
	// at this point.
	if !kicked && client.state > StateClientAuthenticated {
		err := server.broadcastProtoMessage(&mumbleproto.UserRemove{
			Session: proto.Uint32(client.Session()),
		})
		if err != nil {
			server.Panic("Unable to broadcast UserRemove message for disconnected client.")
		}
	}
}

// AddChannel adds a new channel to the server. Automatically assign it a channel ID.
func (server *Server) AddChannel(name string, parent *Channel, isTemp bool) (channel *Channel) {
	channelID := server.nextTempChanID
	if !isTemp {
		parentId := -1
		if parent != nil {
			parentId = parent.Id
		}

		tx := server.DB.Tx()
		ch, err := tx.ChannelAdd(uint64(server.Id), name, int64(parentId))
		if err != nil {
			tx.Rollback()
			return nil
		}
		tx.Commit()
		channelID = int(ch.ChannelID)
	} else {
		server.nextTempChanID += 1
	}

	channel = NewChannel(channelID, name)
	channel.temporary = isTemp
	if parent != nil {
		parent.AddChild(channel)
	}
	server.Channels[channelID] = channel
	return
}

// RemoveChanel removes a channel from the server.
func (server *Server) RemoveChanel(channel *Channel) {
	if channel.Id == 0 {
		server.Printf("Attempted to remove root channel.")
		return
	}

	delete(server.Channels, channel.Id)
	if !channel.IsTemporary() {
		tx := server.DB.Tx()
		err := tx.ChannelRemove(uint64(server.Id), uint64(channel.Id))
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}
}

// Link two channels
func (server *Server) LinkChannels(channel *Channel, other *Channel) {
	channel.Links[other.Id] = other
	other.Links[channel.Id] = channel

	if !channel.IsTemporary() && !other.IsTemporary() {
		tx := server.DB.Tx()
		tx.ChannelLinkAdd(uint64(server.Id), uint64(channel.Id), uint64(other.Id))
		tx.ChannelLinkAdd(uint64(server.Id), uint64(other.Id), uint64(channel.Id))
		tx.Commit()
	}
}

// Unlink two channels
func (server *Server) UnlinkChannels(channel *Channel, other *Channel) {
	delete(channel.Links, other.Id)
	delete(other.Links, channel.Id)

	if !channel.IsTemporary() && !other.IsTemporary() {
		tx := server.DB.Tx()
		tx.ChannelLinkRemove(uint64(server.Id), uint64(channel.Id), uint64(other.Id))
		tx.ChannelLinkRemove(uint64(server.Id), uint64(other.Id), uint64(channel.Id))
		tx.Commit()
	}
}

// This is the synchronous handler goroutine.
// Important control channel messages are routed through this Goroutine
// to keep server state synchronized.
func (server *Server) handlerLoop() {
	regtick := time.Tick(time.Hour)
	for {
		select {
		// We're done. Stop the server's event handler
		case <-server.bye:
			return
		// Control channel messages
		case msg := <-server.incoming:
			server.handleIncomingMessage(msg)
		// Voice broadcast
		case vb := <-server.voicebroadcast:
			server.handleVoiceBroadcast(vb)
		// Remove a temporary channel
		case tempChannel := <-server.tempRemove:
			if tempChannel.IsEmpty() {
				server.RemoveChannel(tempChannel, nil)
			}
		// Finish client authentication. Send post-authentication
		// server info.
		case client := <-server.clientAuthenticated:
			server.finishAuthenticate(client)

		// Server registration update
		// Tick every hour + a minute offset based on the server id.
		case <-regtick:
			server.RegisterPublicServer()
		}
	}
}

// Handle an Authenticate protobuf message.  This is handled in a separate
// goroutine to allow for remote authenticators that are slow to respond.
//
// Once a user has been authenticated, it will ping the server's handler
// routine, which will call the finishAuthenticate method on Server which
// will send the channel tree, user list, etc. to the client.
func (server *Server) handleAuthenticate(client *Client, msg *Message) {
	// Is this message not an authenticate message? If not, discard it...
	if msg.kind != mumbleproto.MessageAuthenticate {
		client.Panic("Unexpected message. Expected Authenticate.")
		return
	}

	auth := &mumbleproto.Authenticate{}
	err := proto.Unmarshal(msg.buf, auth)
	if err != nil {
		client.Panic("Unable to unmarshal Authenticate message.")
		return
	}

	// Set access tokens. Clients can set their access tokens any time
	// by sending an Authenticate message with he contents of their new
	// access token list.
	client.tokens = auth.Tokens
	server.ClearCaches()

	if client.state >= StateClientAuthenticated {
		// todo(jim-k): authenticated user handling is more complex in murmur, check it later.
		return
	}

	// Did we get a username?
	if auth.Username == nil || len(*auth.Username) == 0 {
		client.RejectAuth(mumbleproto.Reject_InvalidUsername, "Please specify a username to log in")
		return
	}

	client.Username = *auth.Username

	tx := server.DB.Tx()
	defer tx.Commit()

	userID := int64(-1)
	dbu, err := tx.UserGetAuth(uint64(server.Id), client.Username)
	if err != nil {
		client.Panicf("Failed to get user info: %v", err)
	}
	if dbu != nil {
		// First try to match with user password
		if dbu.Password.Hash != "" {
			success, err := server.verifyPasswordHash(dbu.Password, *auth.Password)
			if err != nil {
				client.Panicf("Failed to verify password: %v", err)
			}
			if success {
				userID = int64(dbu.UserID)
			} else {
				client.RejectAuth(mumbleproto.Reject_WrongUserPW, "")
				return
			}
		} else if dbu.UserID == 0 {
			// For superuser, empty password is not allowed
			client.RejectAuth(mumbleproto.Reject_WrongUserPW, "")
			return
		}
	}

	// Name match failed, try match with certificate
	if userID < 0 && client.HasCertificate() {
		// Try match by cert
		uid, err := tx.UserInfoGetUID(uint64(server.Id), database.UserHash, client.CertHash())
		if err != nil {
			client.Panicf("Failed to get user info from cert hash: %v", err)
		}
		// If the certificate is verified, try to match with user email
		if !uid.Valid && client.IsCertificateVerified() {
			uid, err = tx.UserInfoGetUID(uint64(server.Id), database.UserEmail, client.Email)
			if err != nil {
				client.Panicf("Failed to get user info from cert hash: %v", err)
			}
		}
		if uid.Valid {
			userID = uid.Int64
			dbu, err = tx.UserGetInfo(uint64(server.Id), uint64(userID))
			if err != nil {
				client.Panicf("Failed to get user info from cert hash: %v", err)
			}
			client.Username = dbu.Name
		}
	}

	if userID >= 0 {
		user, ok := server.Users[uint32(userID)]
		if !ok {
			user = UserFromDatabase(dbu, &tx)
			server.Users[uint32(userID)] = user
		}
		client.user = user
	} else if dbu != nil {
		// Non of user password or certificate check is pass
		client.RejectAuth(mumbleproto.Reject_WrongUserPW, "")
		return
	}

	// Update certificate from hash
	if userID > 0 && client.HasCertificate() {
		info := make(map[database.UserInfoKey]string)
		if len(client.certHash) > 0 {
			info[database.UserHash] = client.certHash
		}
		if len(client.Email) > 0 {
			info[database.UserEmail] = client.Email
		}
		err = tx.UserInfoSet(uint64(server.Id), uint64(userID), info)
		if err != nil {
			server.Printf("Failed to update user cert and email: %v", err)
		}
	}

	if client.user == nil && server.hasServerPassword() {
		if auth.Password == nil || !server.CheckServerPassword(*auth.Password) {
			client.RejectAuth(mumbleproto.Reject_WrongServerPW, "Invalid server password")
			return
		}
	}

	// Setup the cryptstate for the client.
	err = client.crypt.GenerateKey(client.CryptoMode)
	if err != nil {
		client.Panicf("%v", err)
		return
	}

	// Send CryptState information to the client so it can establish an UDP connection,
	// if it wishes.
	client.lastResync = time.Now().Unix()
	err = client.sendMessage(&mumbleproto.CryptSetup{
		Key:         client.crypt.Key,
		ClientNonce: client.crypt.DecryptIV,
		ServerNonce: client.crypt.EncryptIV,
	})
	if err != nil {
		client.Panicf("%v", err)
	}

	// Add codecs
	client.codecs = auth.CeltVersions
	client.opus = auth.GetOpus()

	client.state = StateClientAuthenticated
	client.Bandwidth.ResetIdleSeconds()
	server.clientAuthenticated <- client
}

// The last part of authentication runs in the server's synchronous handler.
func (server *Server) finishAuthenticate(client *Client) {
	// If the client succeeded in proving to the server that it should be granted
	// the credentials of a registered user, do some sanity checking to make sure
	// that user isn't already connected.
	//
	// If the user is already connected, try to check whether this new client is
	// connecting from the same IP address. If that's the case, disconnect the
	// previous client and let the new guy in.
	if client.user != nil {
		found := false
		var oldClient *Client = nil
		for _, connectedClient := range server.clients {
			if connectedClient.UserId() == client.UserId() {
				found = true
				oldClient = connectedClient
				break
			}
		}
		// The user is already present on the server.
		if found {
			if client.tcpaddr.String() != oldClient.tcpaddr.String() {
				client.RejectAuth(mumbleproto.Reject_UsernameInUse, "A client is already connected using those credentials.")
				return
			}
		}

		// No, that user isn't already connected. Move along.
	}

	maxUsers := server.cfg.IntValue("MaxUsers")
	if len(server.Users) >= maxUsers {
		client.RejectAuth(mumbleproto.Reject_ServerFull, fmt.Sprintf("Server is full (max %d users)", maxUsers))
		return
	}

	// Add the client to the connected list
	server.clients[client.Session()] = client

	// Warn clients without CELT support that they might not be able to talk to everyone else.
	if len(client.codecs) == 0 {
		client.codecs = []int32{CeltCompatBitstream}
		server.Printf("Client %v connected without CELT codecs. Faking compat bitstream.", client.Session())
		if server.Opus && !client.opus {
			client.sendMessage(&mumbleproto.TextMessage{
				Session: []uint32{client.Session()},
				Message: proto.String("<strong>WARNING:</strong> Your client doesn't support the CELT codec, you won't be able to talk to or hear most clients. Please make sure your client was built with CELT support."),
			})
		}
	}

	// First, check whether we need to tell the other connected
	// clients to switch to a codec so the new guy can actually speak.
	server.updateCodecVersions(client)

	client.sendChannelList()

	// Add the client to the host slice for its host address.
	host := client.tcpaddr.IP.String()
	server.hmutex.Lock()
	server.hclients[host] = append(server.hclients[host], client)
	server.hmutex.Unlock()

	channel := server.RootChannel()
	if client.IsRegistered() {
		lastChannel, ok := server.Channels[client.user.LastChannelId]
		if ok && acl.HasPermission(&lastChannel.ACL, client, acl.EnterPermission) && !server.IsChannelFull(lastChannel, client) {
			channel = lastChannel
		} else {
			defaultCh, ok := server.Channels[server.cfg.IntValue("DefaultChannel")]
			if ok && acl.HasPermission(&defaultCh.ACL, client, acl.EnterPermission) && !server.IsChannelFull(defaultCh, client) {
				channel = defaultCh
			}
		}
	}

	if server.IsChannelFull(channel, client) {
		client.RejectAuth(mumbleproto.Reject_ServerFull, "Server channels are full")
		return
	}

	userstate := &mumbleproto.UserState{
		Session:   proto.Uint32(client.Session()),
		Name:      proto.String(client.ShownName()),
		ChannelId: proto.Uint32(uint32(channel.Id)),
	}

	if client.HasCertificate() {
		userstate.Hash = proto.String(client.CertHash())
	}

	if client.IsRegistered() {
		userstate.UserId = proto.Uint32(uint32(client.UserId()))

		if client.user.HasTexture() {
			// Does the client support blobs?
			if client.Version.SupportCommentTextureHash() {
				userstate.TextureHash = client.user.TextureBlobHashBytes()
			} else {
				buf, err := blobStore.Get(client.user.TextureBlob)
				if err != nil {
					server.Panicf("Blobstore error: %v", err.Error())
				}
				userstate.Texture = buf
			}
		}

		if client.user.HasComment() {
			// Does the client support blobs?
			if client.Version.SupportCommentTextureHash() {
				userstate.CommentHash = client.user.CommentBlobHashBytes()
			} else {
				buf, err := blobStore.Get(client.user.CommentBlob)
				if err != nil {
					server.Panicf("Blobstore error: %v", err.Error())
				}
				userstate.Comment = proto.String(string(buf))
			}
		}
	}

	server.userEnterChannel(client, channel, userstate)
	if err := server.broadcastProtoMessage(userstate); err != nil {
		// Server panic?
	}

	server.sendUserList(client)

	sync := &mumbleproto.ServerSync{}
	sync.Session = proto.Uint32(client.Session())
	sync.MaxBandwidth = proto.Uint32(server.cfg.Uint32Value("MaxBandwidth"))
	sync.WelcomeText = proto.String(server.cfg.StringValue("WelcomeText"))
	if client.IsSuperUser() {
		sync.Permissions = proto.Uint64(uint64(acl.AllPermissions))
	} else {
		// fixme(mkrautz): previously we calculated the user's
		// permissions and sent them to the client in here. This
		// code relied on our ACL cache, but that has been temporarily
		// thrown out because of our ACL handling code moving to its
		// own package.
		sync.Permissions = nil
	}
	if err := client.sendMessage(sync); err != nil {
		client.Panicf("%v", err)
		return
	}

	err := client.sendMessage(&mumbleproto.ServerConfig{
		AllowHtml:          proto.Bool(server.cfg.BoolValue("AllowHTML")),
		MessageLength:      proto.Uint32(server.cfg.Uint32Value("MaxTextMessageLength")),
		ImageMessageLength: proto.Uint32(server.cfg.Uint32Value("MaxImageMessageLength")),
		MaxUsers:           proto.Uint32(server.cfg.Uint32Value("MaxUsers")),
		RecordingAllowed:   proto.Bool(server.cfg.BoolValue("AllowRecording")),
	})
	if err != nil {
		client.Panicf("%v", err)
		return
	}

	client.state = StateClientReady
	client.clientReady <- true
}

func (server *Server) updateCodecVersions(connecting *Client) {
	codecusers := map[int32]int{}
	var (
		winner     int32
		count      int
		users      int
		opus       int
		enableOpus bool
		txtMsg     *mumbleproto.TextMessage = &mumbleproto.TextMessage{
			Message: proto.String("<strong>WARNING:</strong> Your client doesn't support the Opus codec the server is switching to, you won't be able to talk or hear anyone. Please upgrade to a client with Opus support."),
		}
	)

	for _, client := range server.clients {
		users++
		if client.opus {
			opus++
		}
		for _, codec := range client.codecs {
			codecusers[codec] += 1
		}
	}

	for codec, users := range codecusers {
		if users > count {
			count = users
			winner = codec
		}
		if users == count && codec > winner {
			winner = codec
		}
	}

	var current int32
	if server.PreferAlphaCodec {
		current = server.AlphaCodec
	} else {
		current = server.BetaCodec
	}

	enableOpus = users == opus

	if winner != current {
		if winner == CeltCompatBitstream {
			server.PreferAlphaCodec = true
		} else {
			server.PreferAlphaCodec = !server.PreferAlphaCodec
		}

		if server.PreferAlphaCodec {
			server.AlphaCodec = winner
		} else {
			server.BetaCodec = winner
		}
	} else if server.Opus == enableOpus {
		if server.Opus && connecting != nil && !connecting.opus {
			txtMsg.Session = []uint32{connecting.Session()}
			connecting.sendMessage(txtMsg)
		}
		return
	}

	server.Opus = enableOpus

	err := server.broadcastProtoMessage(&mumbleproto.CodecVersion{
		Alpha:       proto.Int32(server.AlphaCodec),
		Beta:        proto.Int32(server.BetaCodec),
		PreferAlpha: proto.Bool(server.PreferAlphaCodec),
		Opus:        proto.Bool(server.Opus),
	})
	if err != nil {
		server.Printf("Unable to broadcast.")
		return
	}

	if server.Opus {
		for _, client := range server.clients {
			if !client.opus && client.state == StateClientReady {
				txtMsg.Session = []uint32{connecting.Session()}
				err := client.sendMessage(txtMsg)
				if err != nil {
					client.Panicf("%v", err)
				}
			}
		}
		if connecting != nil && !connecting.opus {
			txtMsg.Session = []uint32{connecting.Session()}
			connecting.sendMessage(txtMsg)
		}
	}

	server.Printf("CELT codec switch %#x %#x (PreferAlpha %v) (Opus %v)", uint32(server.AlphaCodec), uint32(server.BetaCodec), server.PreferAlphaCodec, server.Opus)
}

func (server *Server) sendUserList(client *Client) {
	for _, connectedClient := range server.clients {
		if connectedClient.state != StateClientReady {
			continue
		}
		if connectedClient == client {
			continue
		}

		userstate := &mumbleproto.UserState{
			Session:   proto.Uint32(connectedClient.Session()),
			Name:      proto.String(connectedClient.ShownName()),
			ChannelId: proto.Uint32(uint32(connectedClient.Channel.Id)),
		}

		if connectedClient.HasCertificate() {
			userstate.Hash = proto.String(connectedClient.CertHash())
		}

		if connectedClient.IsRegistered() {
			userstate.UserId = proto.Uint32(uint32(connectedClient.UserId()))

			if connectedClient.user.HasTexture() {
				// Does the client support blobs?
				if client.Version.SupportCommentTextureHash() {
					userstate.TextureHash = connectedClient.user.TextureBlobHashBytes()
				} else {
					buf, err := blobStore.Get(connectedClient.user.TextureBlob)
					if err != nil {
						server.Panicf("Blobstore error: %v", err.Error())
					}
					userstate.Texture = buf
				}
			}

			if connectedClient.user.HasComment() {
				// Does the client support blobs?
				if client.Version.SupportCommentTextureHash() {
					userstate.CommentHash = connectedClient.user.CommentBlobHashBytes()
				} else {
					buf, err := blobStore.Get(connectedClient.user.CommentBlob)
					if err != nil {
						server.Panicf("Blobstore error: %v", err.Error())
					}
					userstate.Comment = proto.String(string(buf))
				}
			}
		}

		if connectedClient.Mute {
			userstate.Mute = proto.Bool(true)
		}
		if connectedClient.Suppress {
			userstate.Suppress = proto.Bool(true)
		}
		if connectedClient.SelfMute {
			userstate.SelfMute = proto.Bool(true)
		}
		if connectedClient.SelfDeaf {
			userstate.SelfDeaf = proto.Bool(true)
		}
		if connectedClient.PrioritySpeaker {
			userstate.PrioritySpeaker = proto.Bool(true)
		}
		if connectedClient.Recording {
			userstate.Recording = proto.Bool(true)
		}
		if connectedClient.PluginContext != nil || len(connectedClient.PluginContext) > 0 {
			userstate.PluginContext = connectedClient.PluginContext
		}
		if len(connectedClient.PluginIdentity) > 0 {
			userstate.PluginIdentity = proto.String(connectedClient.PluginIdentity)
		}

		err := client.sendMessage(userstate)
		if err != nil {
			// Server panic?
			continue
		}
	}
}

// Send a client its permissions for channel.
func (server *Server) sendClientPermissions(client *Client, channel *Channel) {
	// No caching for SuperUser
	if client.IsSuperUser() {
		return
	}

	// fixme(mkrautz): re-add when we have ACL caching
	return

	perm := acl.Permission(acl.NonePermission)
	client.sendMessage(&mumbleproto.PermissionQuery{
		ChannelId:   proto.Uint32(uint32(channel.Id)),
		Permissions: proto.Uint32(uint32(perm)),
	})
}

type ClientPredicate func(client *Client) bool

func (server *Server) broadcastProtoMessageWithPredicate(msg interface{}, clientcheck ClientPredicate) error {
	for _, client := range server.clients {
		if !clientcheck(client) {
			continue
		}
		if client.state < StateClientAuthenticated {
			continue
		}
		err := client.sendMessage(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (server *Server) broadcastProtoMessage(msg interface{}) (err error) {
	err = server.broadcastProtoMessageWithPredicate(msg, func(client *Client) bool { return true })
	return
}

func (server *Server) handleIncomingMessage(msg *Message) {
	switch msg.kind {
	case mumbleproto.MessageAuthenticate:
		server.handleAuthenticate(msg.client, msg)
	case mumbleproto.MessagePing:
		server.handlePingMessage(msg.client, msg)
	case mumbleproto.MessageChannelRemove:
		server.handleChannelRemoveMessage(msg.client, msg)
	case mumbleproto.MessageChannelState:
		server.handleChannelStateMessage(msg.client, msg)
	case mumbleproto.MessageUserState:
		server.handleUserStateMessage(msg.client, msg)
	case mumbleproto.MessageUserRemove:
		server.handleUserRemoveMessage(msg.client, msg)
	case mumbleproto.MessageBanList:
		server.handleBanListMessage(msg.client, msg)
	case mumbleproto.MessageTextMessage:
		server.handleTextMessage(msg.client, msg)
	case mumbleproto.MessageACL:
		server.handleAclMessage(msg.client, msg)
	case mumbleproto.MessageQueryUsers:
		server.handleQueryUsers(msg.client, msg)
	case mumbleproto.MessageCryptSetup:
		server.handleCryptSetup(msg.client, msg)
	case mumbleproto.MessageContextAction:
		server.handleContextAction(msg.client, msg)
	case mumbleproto.MessageUserList:
		server.handleUserList(msg.client, msg)
	case mumbleproto.MessageVoiceTarget:
		server.handleVoiceTarget(msg.client, msg)
	case mumbleproto.MessagePermissionQuery:
		server.handlePermissionQuery(msg.client, msg)
	case mumbleproto.MessageUserStats:
		server.handleUserStatsMessage(msg.client, msg)
	case mumbleproto.MessageRequestBlob:
		server.handleRequestBlob(msg.client, msg)
	case mumbleproto.MessageVersion:
		server.handleVersionMessage(msg.client, msg)
	case mumbleproto.MessagePluginDataTransmission:
		server.handlePluginDataTransmission(msg.client, msg)
	}
}

func (server *Server) handleVoiceBroadcast(vb *VoiceBroadcast) {
	if vb.Target() == uint8(mumbleproto.TargetRegularSpeech) { // Current channel
		channel := vb.sender.Channel

		// Send audio to all users that are listening to the channel
		for _, v := range server.listenerManager.GetListenersForChannel(uint32(channel.Id)) {
			vb.AddReceiver(server.clients[v], mumbleproto.ContextListen,
				server.listenerManager.GetVolumeAdjustment(v, uint32(channel.Id)),
			)
		}

		// Send audio to all users in the same channel
		for _, client := range channel.clients {
			vb.AddReceiver(client, mumbleproto.ContextNormal, nil)
		}

		// Send audio to all linked channels the user has speak-permission
		if len(channel.Links) > 0 {
			linked := channel.AllLinks()
			for _, lc := range linked {
				if lc == channel || !acl.HasPermission(&lc.ACL, vb.sender, acl.SpeakPermission) {
					continue
				}
				// ... to all users that are listening to linked channel
				for _, v := range server.listenerManager.GetListenersForChannel(uint32(lc.Id)) {
					vb.AddReceiver(server.clients[v], mumbleproto.ContextListen,
						server.listenerManager.GetVolumeAdjustment(v, uint32(lc.Id)),
					)
				}
				// ... and user in the channel
				for _, client := range lc.clients {
					vb.AddReceiver(client, mumbleproto.ContextNormal, nil)
				}
			}
		}
	} else {
		target, ok := vb.sender.voiceTargets[uint32(vb.Target())]
		if !ok {
			return
		}

		target.SendVoiceBroadcast(vb)
	}

	vb.Broadcast()
}

// Send the content of buf as a UDP packet to addr.
func (s *Server) SendUDP(buf []byte, addr *net.UDPAddr) (err error) {
	_, err = s.udpconn.WriteTo(buf, addr)
	return
}

// Listen for and handle UDP packets.
func (server *Server) udpListenLoop() {
	defer server.netwg.Done()

	buf := make([]byte, UDPPacketSize)
	for {
		nread, remote, err := server.udpconn.ReadFrom(buf)
		if err != nil {
			if isTimeout(err) {
				continue
			} else {
				return
			}
		}

		udpaddr, ok := remote.(*net.UDPAddr)
		if !ok {
			server.Printf("No UDPAddr in read packet. Disabling UDP. (Windows?)")
			return
		}

		// Length 12 is for ping datagrams from the ConnectDialog.
		if nread == 12 {
			readbuf := bytes.NewBuffer(buf)
			var (
				tmp32 uint32
				rand  uint64
			)
			_ = binary.Read(readbuf, binary.BigEndian, &tmp32)
			_ = binary.Read(readbuf, binary.BigEndian, &rand)

			buffer := bytes.NewBuffer(make([]byte, 0, 24))
			_ = binary.Write(buffer, binary.BigEndian, uint32((1<<16)|(2<<8)|2))
			_ = binary.Write(buffer, binary.BigEndian, rand)
			_ = binary.Write(buffer, binary.BigEndian, uint32(len(server.clients)))
			_ = binary.Write(buffer, binary.BigEndian, server.cfg.Uint32Value("MaxUsers"))
			_ = binary.Write(buffer, binary.BigEndian, server.cfg.Uint32Value("MaxBandwidth"))

			err = server.SendUDP(buffer.Bytes(), udpaddr)
			if err != nil {
				return
			}

		} else {
			server.handleUdpPacket(udpaddr, buf[0:nread])
		}
	}
}

func (server *Server) handleUdpPacket(udpaddr *net.UDPAddr, buf []byte) {
	var match *Client
	plain := make([]byte, len(buf))

	// Determine which client sent the the packet.  First, we
	// check the map 'hpclients' in the server struct. It maps
	// a hort-post combination to a client.
	//
	// If we don't find any matches, we look in the 'hclients',
	// which maps a host address to a slice of clients.
	server.hmutex.Lock()
	defer server.hmutex.Unlock()
	client, ok := server.hpclients[udpaddr.String()]
	if ok {
		err := client.crypt.Decrypt(plain, buf)
		if err != nil {
			client.Debugf("unable to decrypt incoming packet, requesting resync: %v", err)
			client.cryptResync()
			return
		}
		match = client
	} else {
		host := udpaddr.IP.String()
		hostclients := server.hclients[host]
		for _, client := range hostclients {
			err := client.crypt.Decrypt(plain[0:], buf)
			if err != nil {
				client.Debugf("unable to decrypt incoming packet, requesting resync: %v", err)
				client.cryptResync()
				return
			} else {
				match = client
			}
		}
		if match != nil {
			match.udpaddr = udpaddr
			server.hpclients[udpaddr.String()] = match
		}
	}

	if match == nil {
		return
	}

	// Resize the plaintext slice now that we know
	// the true encryption overhead.
	plain = plain[:len(plain)-match.crypt.Overhead()]

	match.udp = true
	match.udprecv <- plain
}

// ClearCaches clears the Server's caches
func (server *Server) ClearCaches() {
	for _, client := range server.clients {
		client.ClearCaches()
	}
}

// Helper method for users entering new channels
func (server *Server) userEnterChannel(client *Client, channel *Channel, userstate *mumbleproto.UserState) {
	if client.Channel == channel {
		return
	}

	oldchan := client.Channel
	if oldchan != nil {
		oldchan.RemoveClient(client)
		if oldchan.IsTemporary() && oldchan.IsEmpty() {
			server.tempRemove <- oldchan
		}
	}
	channel.AddClient(client)

	server.ClearCaches()

	if client.IsRegistered() {
		if err := server.UserSetLastChannel(client.user, channel); err != nil {
			server.Panicf("Failed to set user last channel: %v", err)
		}
	}

	canspeak := acl.HasPermission(&channel.ACL, client, acl.SpeakPermission)
	if canspeak == client.Suppress {
		client.Suppress = !canspeak
		userstate.Suppress = proto.Bool(client.Suppress)
	}

	server.sendClientPermissions(client, channel)
	if channel.parent != nil {
		server.sendClientPermissions(client, channel.parent)
	}
}

// Register a client on the server.
func (s *Server) RegisterClient(client *Client) (uid uint32, err error) {
	tx := s.DB.Tx()
	u, err := tx.UserRegister(uint64(s.Id), client.Username)
	if err != nil {
		tx.Rollback()
		return
	}

	user, err := NewUser(uint32(u.UserID), client.Username)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// Grumble can only register users with certificates.
	if !client.HasCertificate() {
		tx.Rollback()
		return 0, errors.New("no cert hash")
	}

	// Save user certficate and email into database
	info := make(map[database.UserInfoKey]string)
	if client.Email != "" {
		info[database.UserEmail] = client.Email
	}
	if client.certHash != "" {
		info[database.UserHash] = client.CertHash()
	}
	err = tx.UserInfoSet(uint64(s.Id), u.UserID, info)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	uid = user.Id
	s.Users[uid] = user

	return uid, nil
}

// RemoveRegistration removes a registered user.
func (s *Server) RemoveRegistration(uid uint32) (err error) {
	_, ok := s.Users[uid]
	if !ok {
		return errors.New("unknown user ID")
	}

	tx := s.DB.Tx()
	err = tx.UserUnregister(uint64(s.Id), uint64(uid))
	if err != nil {
		tx.Rollback()
		return
	}
	tx.Commit()

	// Remove from user maps
	delete(s.Users, uid)

	// Remove from groups and ACLs.
	s.removeRegisteredUserFromChannel(uid, s.RootChannel())

	return nil
}

// Remove references for user id uid from channel. Traverses subchannels.
func (s *Server) removeRegisteredUserFromChannel(uid uint32, channel *Channel) {

	newACL := []acl.ACL{}
	for _, chanacl := range channel.ACL.ACLs {
		if chanacl.UserId == int(uid) {
			continue
		}
		newACL = append(newACL, chanacl)
	}
	channel.ACL.ACLs = newACL

	for _, grp := range channel.ACL.Groups {
		delete(grp.Add, int(uid))
		delete(grp.Remove, int(uid))
		delete(grp.Temporary, int(uid))
	}

	for _, subChan := range channel.children {
		s.removeRegisteredUserFromChannel(uid, subChan)
	}
}

// RemoveChannel removes a channel
func (server *Server) RemoveChannel(channel *Channel, dest *Channel) {
	// Can't remove root
	if channel == server.RootChannel() {
		return
	}

	if dest == nil {
		dest = channel.parent
	}

	// Remove all subchannels
	for _, subChannel := range channel.children {
		server.RemoveChannel(subChannel, dest)
	}

	tx := server.DB.Tx()
	defer tx.Commit()

	// Remove all links
	for _, linkedChannel := range channel.Links {
		err := tx.ChannelLinkRemove(uint64(server.Id), uint64(linkedChannel.Id), uint64(channel.Id))
		if err != nil {
			tx.Rollback()
			server.Panicf("Failed to remove channel: %v", err)
		}

		delete(linkedChannel.Links, channel.Id)
	}

	// Remove all clients
	for _, client := range channel.clients {
		target := dest
		for target.parent != nil && (!acl.HasPermission(&target.ACL, client, acl.EnterPermission) || server.IsChannelFull(target, client)) {
			target = target.parent
		}

		userstate := &mumbleproto.UserState{}
		userstate.Session = proto.Uint32(client.Session())
		userstate.ChannelId = proto.Uint32(uint32(target.Id))
		server.userEnterChannel(client, target, userstate)
		if err := server.broadcastProtoMessage(userstate); err != nil {
			server.Panicf("%v", err)
		}
	}

	// Remove from database
	err := tx.ChannelRemove(uint64(server.Id), uint64(channel.Id))
	if err != nil {
		tx.Rollback()
		server.Panicf("Failed to remove channel: %v", err)
	}

	// Remove the channel itself
	parent := channel.parent
	delete(parent.children, channel.Id)
	delete(server.Channels, channel.Id)
	chanremove := &mumbleproto.ChannelRemove{
		ChannelId: proto.Uint32(uint32(channel.Id)),
	}
	if err := server.broadcastProtoMessage(chanremove); err != nil {
		server.Panicf("%v", err)
	}
}

func (server *Server) readBanList() {
	err := server.DB.Transaction(func(tx *database.DbTx) error {
		bans, err := tx.BanRead(uint64(server.Id))
		if err != nil {
			return err
		}
		server.Bans = make([]ban.Ban, len(bans))
		for i, v := range bans {
			server.Bans[i] = ban.Ban{
				IP:       v.Base,
				Mask:     v.Mask,
				Username: v.Name,
				CertHash: string(v.Hash),
				Reason:   v.Reason,
				Start:    v.Start.Unix(),
				Duration: uint32(v.Duraion),
			}
		}
		return nil
	})
	if err != nil {
		server.Panic(err)
	}
}

func (server *Server) saveBanList() {
	banList := make([]database.Ban, len(server.Bans))
	for i, v := range server.Bans {
		banList[i] = database.Ban{
			Base:    v.IP,
			Mask:    v.Mask,
			Name:    v.Username,
			Hash:    []byte(v.CertHash),
			Reason:  v.Reason,
			Start:   time.Unix(v.Start, 0),
			Duraion: int(v.Duration),
		}
	}

	err := server.DB.Transaction(func(tx *database.DbTx) error {
		return tx.BanWrite(banList)
	})
	if err != nil {
		server.Panic(err)
		return
	}
}

// RemoveExpiredBans removes expired bans
func (server *Server) RemoveExpiredBans() {
	server.banlock.Lock()
	defer server.banlock.Unlock()

	newBans := []ban.Ban{}
	update := false
	for _, ban := range server.Bans {
		if !ban.IsExpired() {
			newBans = append(newBans, ban)
		} else {
			update = true
		}
	}

	if update {
		server.Bans = newBans
		server.saveBanList()
	}
}

// IsConnectionBanned Is the incoming connection conn banned?
func (server *Server) IsConnectionBanned(conn net.Conn) bool {
	server.banlock.RLock()
	defer server.banlock.RUnlock()

	for _, ban := range server.Bans {
		addr := conn.RemoteAddr().(*net.TCPAddr)
		if ban.Match(addr.IP) && !ban.IsExpired() {
			return true
		}
	}

	return false
}

// IsCertHashBanned Is the certificate hash banned?
func (server *Server) IsCertHashBanned(hash string) bool {
	server.banlock.RLock()
	defer server.banlock.RUnlock()

	for _, ban := range server.Bans {
		if ban.CertHash == hash && !ban.IsExpired() {
			return true
		}
	}

	return false
}

// Filter incoming text according to the server's current rules.
func (server *Server) FilterText(text string) (filtered string, err error) {
	options := &htmlfilter.Options{
		StripHTML:             !server.cfg.BoolValue("AllowHTML"),
		MaxTextMessageLength:  server.cfg.IntValue("MaxTextMessageLength"),
		MaxImageMessageLength: server.cfg.IntValue("MaxImageMessageLength"),
	}
	return htmlfilter.Filter(text, options)
}

// UpdateChannel save the channel state into database
func (server *Server) UpdateChannel(channel *Channel) (err error) {
	if channel.IsTemporary() {
		return nil
	}

	sid := uint64(server.Id)
	cid := uint64(channel.Id)

	tx := server.DB.Tx()
	defer tx.Commit()

	parentID := -1
	if channel.parent != nil {
		parentID = channel.parent.Id
	}
	err = tx.ChannelUpdate(sid, cid, channel.Name, int64(parentID), channel.ACL.InheritACL)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.ChannelInfoSet(sid, cid, map[database.ChannelInfoKey]string{
		database.ChannelDescription: channel.DescriptionBlob,
		database.ChannelPosition:    strconv.Itoa(channel.Position),
		database.ChannelMaxUsers:    strconv.Itoa(channel.MaxUsers),
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.GroupDeleteByChannel(sid, cid)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.ACLRemoveByChannel(sid, cid)
	if err != nil {
		tx.Rollback()
		return err
	}

	for name, group := range channel.ACL.Groups {
		g, err := tx.GroupAdd(sid, cid, name, group.Inherit, group.Inheritable)
		if err != nil {
			tx.Rollback()
			return err
		}

		for uid := range group.Add {
			err = tx.GroupMemberAdd(sid, g.GroupID, uint64(uid), true)
			if err != nil {
				tx.Rollback()
				return err
			}
		}

		for uid := range group.Remove {
			err = tx.GroupMemberAdd(sid, g.GroupID, uint64(uid), false)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	priority := 5
	for _, acl := range channel.ACL.ACLs {
		dbacl := database.NewACL(sid, cid, priority).Apply(acl.ApplyHere, acl.ApplySubs).Grant(int(acl.Allow)).Revoke(int(acl.Deny))
		if acl.UserId >= 0 {
			dbacl = dbacl.WithUser(uint64(acl.UserId))
		}
		if acl.Group != "" {
			dbacl = dbacl.WithGroup(acl.Group)
		}

		err = tx.ACLAdd(dbacl)
		if err != nil {
			tx.Rollback()
			return err
		}

		priority++
	}

	return nil
}

// The accept loop of the server.
func (server *Server) acceptLoop(listener net.Listener) {
	defer server.netwg.Done()

	for {
		// New client connected
		conn, err := listener.Accept()
		if err != nil {
			if isTimeout(err) {
				continue
			} else {
				return
			}
		}

		// Remove expired bans
		server.RemoveExpiredBans()

		// Is the client IP-banned?
		if server.IsConnectionBanned(conn) {
			server.Printf("Rejected client %v: Banned", conn.RemoteAddr())
			err := conn.Close()
			if err != nil {
				server.Printf("Unable to close connection: %v", err)
			}
			continue
		}

		// Create a new client connection from our *tls.Conn
		// which wraps net.TCPConn.
		err = server.handleIncomingClient(conn)
		if err != nil {
			server.Printf("Unable to handle new client: %v", err)
			continue
		}
	}
}

// The isTimeout function checks whether a
// network error is a timeout.
func isTimeout(err error) bool {
	if e, ok := err.(net.Error); ok {
		return e.Timeout()
	}
	return false
}

// Initialize the per-launch data
func (server *Server) initPerLaunchData() {
	server.pool = sessionpool.New()
	server.clients = make(map[uint32]*Client)
	server.hclients = make(map[string][]*Client)
	server.hpclients = make(map[string]*Client)

	server.bye = make(chan bool)
	server.incoming = make(chan *Message)
	server.voicebroadcast = make(chan *VoiceBroadcast)
	server.tempRemove = make(chan *Channel, 1)
	server.clientAuthenticated = make(chan *Client)
}

// Clean per-launch data
func (server *Server) cleanPerLaunchData() {
	server.pool = nil
	server.clients = nil
	server.hclients = nil
	server.hpclients = nil

	server.bye = nil
	server.incoming = nil
	server.voicebroadcast = nil
	server.tempRemove = nil
	server.clientAuthenticated = nil
}

// Port returns the port the native server will listen on when it is
// started.
func (server *Server) Port() int {
	port := server.cfg.IntValue("Port")
	if port == 0 {
		return DefaultPort + int(server.Id) - 1
	}
	return port
}

// ListenWebPort returns true if we should listen to the
// web port, otherwise false
func (server *Server) ListenWebPort() bool {
	return !server.cfg.BoolValue("NoWebServer")
}

// WebPort returns the port the web server will listen on when it is
// started.
func (server *Server) WebPort() int {
	port := server.cfg.IntValue("WebPort")
	if port == 0 {
		return DefaultWebPort + int(server.Id) - 1
	}
	return port
}

// CurrentPort returns the port the native server is currently listening
// on.  If called when the server is not running,
// this function returns -1.
func (server *Server) CurrentPort() int {
	if !server.running {
		return -1
	}
	tcpaddr := server.tcpl.Addr().(*net.TCPAddr)
	return tcpaddr.Port
}

// HostAddress returns the host address the server will listen on when
// it is started. This must be an IP address, either IPv4
// or IPv6.
func (server *Server) HostAddress() string {
	host := server.cfg.StringValue("Address")
	if host == "" {
		return "0.0.0.0"
	}
	return host
}

// Start the server.
func (server *Server) Start() (err error) {
	if server.running {
		return errors.New("already running")
	}

	host := server.HostAddress()
	port := server.Port()
	webport := server.WebPort()
	shouldListenWeb := server.ListenWebPort()

	// Setup our UDP listener
	server.udpconn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		return err
	}
	/*
		err = server.udpconn.SetReadTimeout(1e9)
		if err != nil {
			return err
		}
	*/

	// Set up our TCP connection
	server.tcpl, err = net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		return err
	}
	/*
		err = server.tcpl.SetTimeout(1e9)
		if err != nil {
			return err
		}
	*/

	// Wrap a TLS listener around the TCP connection
	certFn := filepath.Join(server.DataDir, "cert.pem")
	keyFn := filepath.Join(server.DataDir, "key.pem")
	cert, err := tls.LoadX509KeyPair(certFn, keyFn)
	if err != nil {
		return err
	}
	server.tlscfg = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequestClientCert,
	}
	server.tlsl = tls.NewListener(server.tcpl, server.tlscfg)

	if shouldListenWeb {
		// Create HTTP server and WebSocket "listener"
		webaddr := &net.TCPAddr{IP: net.ParseIP(host), Port: webport}
		server.webtlscfg = &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
			NextProtos:   []string{"http/1.1"},
		}
		server.webwsl = web.NewListener(webaddr, server.Logger)
		mux := http.NewServeMux()
		mux.Handle("/", server.webwsl)
		server.webhttp = &http.Server{
			Addr:      webaddr.String(),
			Handler:   mux,
			TLSConfig: server.webtlscfg,
			ErrorLog:  server.Logger,

			// Set sensible timeouts, in case no reverse proxy is in front of Grumble.
			// Non-conforming (or malicious) clients may otherwise block indefinitely and cause
			// file descriptors (or handles, depending on your OS) to leak and/or be exhausted
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  2 * time.Minute,
		}
		go func() {
			err := server.webhttp.ListenAndServeTLS("", "")
			if err != http.ErrServerClosed {
				server.Fatalf("Fatal HTTP server error: %v", err)
			}
		}()

		server.Printf("Started: listening on %v and %v", server.tcpl.Addr(), server.webwsl.Addr())
	} else {
		server.Printf("Started: listening on %v", server.tcpl.Addr())
	}

	server.running = true

	// Reset the server's per-launch data to
	// a clean state.
	server.initPerLaunchData()

	// Launch the event handler goroutine
	go server.handlerLoop()

	// Add the three network receiver goroutines to the net waitgroup
	// and launch them.
	//
	// We use the waitgroup to provide a blocking Stop() method
	// for the servers. Each network goroutine defers a call to
	// netwg.Done(). In the Stop() we close all the connections
	// and call netwg.Wait() to wait for the goroutines to end.
	numWG := 2
	if shouldListenWeb {
		numWG++
	}

	server.netwg.Add(numWG)
	go server.udpListenLoop()
	go server.acceptLoop(server.tlsl)
	if shouldListenWeb {
		go server.acceptLoop(server.webwsl)
	}

	// Schedule a server registration update (if needed)
	go func() {
		time.Sleep(1 * time.Minute)
		server.RegisterPublicServer()
	}()

	return nil
}

// Stop the server.
func (server *Server) Stop() (err error) {
	if !server.running {
		return errors.New("server not running")
	}

	// Stop the handler goroutine and disconnect all
	// clients
	server.bye <- true
	for _, client := range server.clients {
		client.Disconnect()
	}

	if server.ListenWebPort() {
		// Wait for the HTTP server to shutdown gracefully
		// A client could theoretically block the server from ever stopping by
		// never letting the HTTP connection go idle, so we give 15 seconds of grace time.
		// This does not apply to opened WebSockets, which were forcibly closed when
		// all clients were disconnected.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
		err = server.webhttp.Shutdown(ctx)
		cancel()
		if err == context.DeadlineExceeded {
			server.Println("Forcibly shutdown HTTP server while stopping")
		} else if err != nil {
			return err
		}

		err = server.webwsl.Close()
		if err != nil {
			return err
		}
	}

	// Close the listeners
	err = server.tlsl.Close()
	if err != nil {
		return err
	}

	// Close the UDP connection
	err = server.udpconn.Close()
	if err != nil {
		return err
	}

	// Wait for the three network receiver
	// goroutines end.
	server.netwg.Wait()

	server.cleanPerLaunchData()
	server.running = false
	server.Printf("Stopped")

	return nil
}

// Set will set a configuration value
func (server *Server) Set(key string, value string) {
	server.cfg.Set(key, value)
}

// ValidateUsername will check if specified user name is validate
func (server *Server) ValidateUsername(name string) bool {
	return len(name) > 0 && len(name) < 128
}

// ValidateChannelName will check if specified channel name is validate
func (server *Server) ValidateChannelName(name string) bool {
	return len(name) > 0 && len(name) < 128
}

// CanNest check if a channel can be set as a parent
func (server *Server) CanNest(parent, channel *Channel) bool {
	parentLevel := -1
	channelDepth := 0
	if parent != nil {
		parentLevel = parent.Level()
	}
	if channel != nil {
		channelDepth = channel.Depth()
	}

	limit := server.cfg.IntValue("ChannelNestingLimit")
	return parentLevel+channelDepth < limit
}

func (server *Server) ChannelReachLimit() bool {
	limit := server.cfg.IntValue("ChannelCountLimit")
	if limit > 0 && len(server.Channels) >= limit {
		return true
	}
	return false
}

func (server *Server) IsChannelFull(channel *Channel, user acl.User) bool {
	if user != nil && acl.HasPermission(&channel.ACL, user, acl.WritePermission) {
		return false
	}
	if channel.MaxUsers > 0 {
		return len(channel.clients) >= channel.MaxUsers
	}
	globalLimit := server.cfg.IntValue("MaxUsersPerChannel")
	if globalLimit > 0 {
		return len(channel.clients) >= globalLimit
	}
	return false
}

func (server *Server) SetChannelListenerVolume(client *Client, channel *Channel, volume float32) error {
	if client.user == nil {
		return nil
	}

	tx := server.DB.Tx()
	defer tx.Commit()

	return tx.ChannelListenerSetVolume(uint64(server.Id), uint64(client.UserId()), uint64(channel.Id), volume)
}

func (server *Server) AddChannelListener(client *Client, channel *Channel) error {
	if client.user == nil {
		return nil
	}

	tx := server.DB.Tx()
	defer tx.Commit()

	_, err := tx.ChannelListenerAdd(uint64(server.Id), uint64(client.UserId()), uint64(channel.Id))
	return err
}

func (server *Server) DisableChannelListener(client *Client, channel *Channel) error {
	if client.user == nil {
		return nil
	}

	tx := server.DB.Tx()
	defer tx.Commit()

	err := tx.ChannelListenerSetEnabled(uint64(server.Id), uint64(client.UserId()), uint64(channel.Id), false)
	return err
}
