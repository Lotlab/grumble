package main

import "math"

type VolumeAdjustment struct {
	Factor       float32
	DBAdjustment int
}

const InvalidDbAdjustment = 2147483647

func volumeFactorToDbFloat(factor float32) float32 {
	return float32(math.Log2(float64(factor)) * 6.0)
}

func volumeDbAdjustToFactor(db int) float32 {
	return float32(math.Pow(2.0, float64(db)/6.0))
}

func DefaultVolumeAdjustment() VolumeAdjustment {
	return VolumeAdjustment{
		Factor:       1,
		DBAdjustment: 0,
	}
}

func VolumeAdjustmentFromFactor(factor float32) VolumeAdjustment {
	if factor > 0 {
		db := volumeFactorToDbFloat(factor)
		if math.Abs(float64(db)-float64(int(db))) < 0.1 {
			return VolumeAdjustment{
				Factor:       factor,
				DBAdjustment: int(math.Round(float64(db))),
			}
		}
	}
	return VolumeAdjustment{
		Factor:       factor,
		DBAdjustment: InvalidDbAdjustment,
	}
}

func VolumeAdjustmentFromDb(dbAdjustment int) VolumeAdjustment {
	factor := volumeDbAdjustToFactor(dbAdjustment)
	return VolumeAdjustment{
		Factor:       factor,
		DBAdjustment: dbAdjustment,
	}
}

func (v VolumeAdjustment) Equal(other VolumeAdjustment) bool {
	return v.DBAdjustment == other.DBAdjustment && math.Abs(float64(v.Factor)-float64(other.Factor)) < 0.1
}
