//go:build ignore

package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
)

var replacements = []string{
	`(?m)^package MumbleProto;$`, "package mumbleproto;\noption go_package = \"mumble.info/grumble/pkg/mumbleproto\";",

	// Add crypto_modes to Version message.
	// It is only present in Grumble, not in upstream Murmur.
	`(?m)^(message Version {)$`, "$1\n\trepeated string crypto_modes = 10;\n",
}

func main() {
	downloadProto("https://raw.githubusercontent.com/mumble-voip/mumble/master/src/Mumble.proto",
		"Mumble.proto", "mumble.info/grumble/pkg/mumbleproto", []string{
			// Add crypto_modes to Version message.
			// It is only present in Grumble, not in upstream Murmur.
			`(?m)^(message Version {)$`, "$1\n\trepeated string crypto_modes = 10;\n",
		})

	downloadProto("https://raw.githubusercontent.com/mumble-voip/mumble/master/src/MumbleUDP.proto",
		"MumbleUDP.proto", "mumble.info/grumble/pkg/mumbleproto", []string{
			// Add "UDP" suffix to message of MumbleUDP to avoid name collision
			`(?m)^message (.+) {$`, "message ${1}UDP {",
		})
}

func downloadProto(url, filename string, pkg string, replacements []string) {
	// Fetch proto
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	replacements = append(replacements, `(?m)^package (.+);$`, "package $1;\noption go_package = \""+pkg+"\";")

	// Perform replacements
	for i := 0; i < len(replacements); i += 2 {
		re, rp := replacements[i], replacements[i+1]
		regex, err := regexp.Compile(re)
		if err != nil {
			log.Fatal(err)
		}
		data = regex.ReplaceAll(data, []byte(rp))
	}

	// Write Mumble.proto
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Fatal(err)
	}

	// Run protobuf compiler
	if err := exec.Command("protoc", "--go_out=.", "--go_opt=paths=source_relative", filename).Run(); err != nil {
		log.Fatal(err)
	}
}
