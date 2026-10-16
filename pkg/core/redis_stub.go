//go:build noredis

package core

import (
	"fmt"
	"time"
)

func InitRedis(addr, password string, db int) {}

func (r *Runtime) EnsureRedisFromEnv() bool {
	return false
}

func (r *Runtime) executeRedisMethod(instance *Instance, method string, args []interface{}) interface{} {
	fmt.Println("[Redis] Soporte Redis omitido en este binario optimizado (noredis).")
	return nil
}

func InitRedisSessions(env map[string]string) error {
	return fmt.Errorf("redis session driver omitido en este binario optimizado (noredis)")
}

func GetRedisSession(sessionID string) (string, bool, error) {
	return "", false, fmt.Errorf("redis session storage is not available (noredis)")
}

func SetRedisSession(sessionID string, encoded []byte, ttl time.Duration) error {
	return fmt.Errorf("redis session storage is not available (noredis)")
}
