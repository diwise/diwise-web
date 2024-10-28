package main

import (
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/authn"
	"github.com/diwise/service-chassis/pkg/infrastructure/servicerunner"
)

type FlagType int
type FlagMap map[FlagType]string

const (
	listenAddress FlagType = iota
	servicePort
	controlPort

	devModeEnabled
	webAssetPath

	appRoot
	adminURL
	alarmsURL
	devMgmtURL
	thingsURL
	measurementsURL

	oauth2RealmURL
	oauth2ClientID
	oauth2ClientSecret
)

type AppConfig struct {
	app *application.App
	pte authn.PhantomTokenExchange
}

var ifnot = servicerunner.IfNot[AppConfig]
var onstarting = servicerunner.OnStarting[AppConfig]
var onrunning = servicerunner.OnRunning[AppConfig]
var onshutdown = servicerunner.OnShutdown[AppConfig]
var webserver = servicerunner.WithHTTPServeMux[AppConfig]
var muxinit = servicerunner.OnMuxInit[AppConfig]
var listen = servicerunner.WithListenAddr[AppConfig]
var port = servicerunner.WithPort[AppConfig]
var pprof = servicerunner.WithPPROF[AppConfig]
var liveness = servicerunner.WithK8SLivenessProbe[AppConfig]
var readiness = servicerunner.WithK8SReadinessProbes[AppConfig]
