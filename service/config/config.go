package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

type Config struct {
	TconfigdUrl              *url.URL
	TconfigdSpiffeId         spiffeid.ID
	ServicePort              *int
	InterceptionMode         bool
	AgentHttpsApiPort        int
	AgentHttpApiPort         int
	AgentInterceptorPort     int
	HeartBeatIntervalMinutes int
	MyNamespace              string
}

func GetAppConfig() *Config {
	return &Config{
		TconfigdUrl:              parseURL(getEnv("TCONFIGD_URL")),
		TconfigdSpiffeId:         spiffeid.RequireFromString(getEnv("TCONFIGD_SPIFFE_ID")),
		ServicePort:              getOptionalEnvAsInt("SERVICE_PORT"),
		InterceptionMode:         getEnvAsBool("INTERCEPTION_MODE"),
		AgentHttpsApiPort:        getEnvAsInt("AGENT_HTTPS_API_PORT"),
		AgentHttpApiPort:         getEnvAsInt("AGENT_HTTP_API_PORT"),
		AgentInterceptorPort:     getEnvAsInt("AGENT_INTERCEPTOR_PORT"),
		HeartBeatIntervalMinutes: getEnvAsInt("HEARTBEAT_INTERVAL_MINUTES"),
		MyNamespace:              getEnv("MY_NAMESPACE"),
	}
}

func getEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		panic(fmt.Sprintf("%s environment variable not set", key))
	}

	return value
}

func getEnvAsInt(key string) int {
	valueStr := getEnv(key)
	valueInt, err := strconv.Atoi(valueStr)

	if err != nil {
		panic(fmt.Sprintf("Error converting %s to integer: %v", key, err))
	}

	return valueInt
}

func getOptionalEnvAsInt(key string) *int {
	valueStr, exists := os.LookupEnv(key)
	if !exists || valueStr == "" {
		return nil
	}

	valueInt, err := strconv.Atoi(valueStr)
	if err != nil {
		panic(fmt.Sprintf("Error converting %s to integer: %v", key, err))
	}

	return &valueInt
}

func getEnvAsBool(key string) bool {
	valueStr := getEnv(key)
	valueBool, err := strconv.ParseBool(valueStr)

	if err != nil {
		panic(fmt.Sprintf("Error converting %s to bool: %v", key, err))
	}

	return valueBool
}

func parseURL(rawurl string) *url.URL {
	parsedURL, err := url.Parse(rawurl)
	if err != nil {
		panic(fmt.Sprintf("Error parsing URL %s: %v", rawurl, err))
	}

	return parsedURL
}
