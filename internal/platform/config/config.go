// Package config loads environment configuration without printing secret values.
package config

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AttachmentMaxBytes int64
	LegacyStaticURL    string
	SessionNamespace   string
	DSN                string
	RedisAddress       string
	RedisPassword      string
	ConsulAddress      string
	RPCAddress         string
	GatewayKey         string
	AppKeys            map[string]string
	HTTPAddress        string
	StorageEndpoint    string
	StorageAccessKey   string
	StorageSecretKey   string
	StorageBucket      string
	StaticURL          string
	AdminPhone         string
	AdminPassword      string
	TLSCert            string
	TLSKey             string
	TLSCA              string
	StorageTLS         bool
	CORSOrigins        []string
}

func LoadEnv(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(k, "KERTHUS_") {
			return fmt.Errorf("invalid configuration line")
		}
		if _, set := os.LookupEnv(k); !set {
			if e = os.Setenv(k, v); e != nil {
				return e
			}
		}
	}
	return sc.Err()
}
func Load() (Config, error) {
	var c Config
	envFile := os.Getenv("KERTHUS_ENV_FILE")
	if envFile != "" {
		if e := LoadEnv(envFile); e != nil {
			return c, e
		}
	}
	get := func(k string) string { return os.Getenv("KERTHUS_" + k) }
	c = Config{LegacyStaticURL: get("LEGACY_STATIC_URL"), SessionNamespace: get("SESSION_NAMESPACE"), DSN: get("MYSQL_DSN"), RedisAddress: get("REDIS_ADDRESS"), RedisPassword: get("REDIS_PASSWORD"), ConsulAddress: get("CONSUL_ADDRESS"), RPCAddress: get("RPC_ADDRESS"), GatewayKey: get("RPC_GATEWAY_KEY"), HTTPAddress: get("HTTP_ADDRESS"), StorageEndpoint: get("STORAGE_ENDPOINT"), StorageAccessKey: get("STORAGE_ACCESS_KEY"), StorageSecretKey: get("STORAGE_SECRET_KEY"), StorageBucket: get("STORAGE_BUCKET"), StaticURL: get("STATIC_URL"), AdminPhone: get("ADMIN_PHONE"), AdminPassword: get("ADMIN_PASSWORD"), TLSCert: get("RPC_TLS_CERT"), TLSKey: get("RPC_TLS_KEY"), TLSCA: get("RPC_TLS_CA"), CORSOrigins: strings.Split(get("CORS_ORIGINS"), ",")}
	c.AttachmentMaxBytes = 50 << 20
	if raw := get("ATTACHMENT_MAX_BYTES"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1<<20 || n > 1<<30 {
			return c, fmt.Errorf("KERTHUS_ATTACHMENT_MAX_BYTES must be between 1048576 and 1073741824")
		}
		c.AttachmentMaxBytes = n
	}
	c.StorageTLS, _ = strconv.ParseBool(get("STORAGE_TLS"))
	if raw := get("RPC_APP_KEYS"); raw != "" {
		if e := json.Unmarshal([]byte(raw), &c.AppKeys); e != nil {
			return c, fmt.Errorf("invalid application credential map")
		}
	}
	if c.DSN == "" || c.RedisAddress == "" || c.ConsulAddress == "" || c.RPCAddress == "" || c.HTTPAddress == "" || len(c.GatewayKey) < 32 {
		return c, fmt.Errorf("missing configuration; initialize dev environment or supply KERTHUS_* variables")
	}
	if c.TLSCA == "" {
		host, _, e := net.SplitHostPort(c.RPCAddress)
		ip := net.ParseIP(host)
		if e != nil || ip == nil || !ip.IsLoopback() {
			return c, fmt.Errorf("RPC requires TLS outside loopback")
		}
	}
	return c, nil
}
func (c Config) RPCServerTLS() (*tls.Config, error) {
	if c.TLSCA == "" {
		return nil, nil
	}
	cert, e := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
	if e != nil {
		return nil, e
	}
	pool, e := caPool(c.TLSCA)
	if e != nil {
		return nil, e
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert}, nil
}
func (c Config) RPCClientTLS() (*tls.Config, error) {
	if c.TLSCA == "" {
		return nil, nil
	}
	cert, e := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
	if e != nil {
		return nil, e
	}
	pool, e := caPool(c.TLSCA)
	if e != nil {
		return nil, e
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, RootCAs: pool}, nil
}
func caPool(path string) (*x509.CertPool, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("invalid RPC CA")
	}
	return pool, nil
}
