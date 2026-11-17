package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func Digest(source Source) string {
	source.Keys = nil
	source.Checks = nil
	if source.Moves == nil {
		source.Moves = []string{}
	}
	data, _ := json.Marshal(source)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func CacheKey(j Job) string {
	return fmt.Sprintf("%s:%s:%s:%d:%d:%d:%d", j.Digest, j.Engine, j.Profile, j.Budget.Depth, j.Budget.Millis, j.Budget.HashMiB, j.Budget.Nodes)
}

func ProfileBudget(profile string) Budget {
	if profile == "deep" {
		return Budget{Depth: 8, Millis: 3000, HashMiB: 128}
	}
	return Budget{Depth: 6, Millis: 750, HashMiB: 128}
}
