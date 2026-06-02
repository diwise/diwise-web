package env

import (
	"context"
	"fmt"
	"os"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func GetVariableOrDefault(ctx context.Context, envVar, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetVariableOrDie(ctx context.Context, envVar, description string) string {
	value := os.Getenv(envVar)
	if value == "" {
		logger := logging.GetFromContext(ctx)
		msg := fmt.Sprintf("please set %s to a valid %s.", envVar, description)
		logger.Error(msg)
		panic(msg)
	}
	return value
}
