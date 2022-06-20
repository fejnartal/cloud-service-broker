package packer

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	cp "github.com/otiai10/copy"
	"github.com/spf13/viper"
)

const (
	userHomeDir  = "user.home.dir"
	pakCachePath = "pak.cache_path"
)

func init() {
	viper.BindEnv(userHomeDir, "HOME")
	viper.BindEnv(pakCachePath, "PAK_BUILD_CACHE_PATH")
}

func copyFromCache(cachePath string, source string, destination string) bool {
	if cachePath == "" {
		return false
	}
	cacheKey := buildCacheKey(cachePath, source)
	if _, err := os.Stat(cacheKey); err == nil {
		cp.Copy(cacheKey, destination)
		log.Println("\t", source, "found in cache at", cacheKey)
		return true
	} else {
		return false
	}
}

func populateCache(cachePath string, source string, destination string) {
	if cachePath == "" {
		return
	}
	cacheKey := buildCacheKey(cachePath, source)
	err := cp.Copy(destination, cacheKey)
	if err != nil {
		panic(err)
	}
}

func buildCacheKey(cachePath string, source string) string {
	return path.Join(cachePath, fmt.Sprintf("%x", md5.Sum([]byte(source))))
}

func cachedFetchFile(getter func(source string, destination string) error, source, destination string) error {
	cachePath := readCachePath()
	if cachePath == "" {
		return getter(source, destination)
	}

	if copyFromCache(cachePath, source, destination) {
		return nil
	}
	err := getter(source, destination)
	if err != nil {
		return err
	}
	populateCache(cachePath, source, destination)
	return nil
}

func readCachePath() string {
	switch {
	case viper.IsSet(pakCachePath):
		return viper.GetString(pakCachePath)
	case viper.IsSet(userHomeDir):
		return filepath.Join(viper.GetString(userHomeDir), ".csb-pak-cache")
	default:
		return ""
	}
}
