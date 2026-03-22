package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tamir-liebermann/gobank/api"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	apiMgr := api.NewApiManager()
	apiMgr.StartAlertPoller()
	apiMgr.Run()
}
