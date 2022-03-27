package api

import (
	"fmt"
	"nfgt-client/pkg/client"
	"nfgt-client/pkg/common"

	"github.com/gin-gonic/gin"
)

type RouterController struct {
	client *client.Client
	engine *gin.Engine
	config *common.Config
}

func NewRouterController(config *common.Config, client *client.Client) *RouterController {
	common.Infof("Starting router controller\n")
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	routerController := RouterController{
		client: client,
		engine: engine,
		config: config,
	}

	routerController.registerRoutes()

	return &routerController
}

func (r *RouterController) registerRoutes() {
	api := r.engine.Group("/api")
	{
		transaction := api.Group("/transaction")
		{
			transaction.POST("/create", r.createTransaction)
			transaction.GET(fmt.Sprintf("/status/:%v", TRANSACTION_PARAM), r.checkTransaction)
		}

		query := api.Group("/query")
		{
			spot := query.Group("/spot")
			{
				spot.GET(fmt.Sprintf("/owner/:%v", OWNER_PARAM), r.getOwnerSpot)
				spot.GET(fmt.Sprintf("/asset/:%v", ASSET_PARAM), r.getAssetSpot)
			}

			history := query.Group("/history")
			{
				history.GET(fmt.Sprintf("/owner/:%v/:%v", OWNER_PARAM, DEPTH_PARAM), r.getOwnerHistory)
				history.GET(fmt.Sprintf("/asset/:%v/:%v", ASSET_PARAM, DEPTH_PARAM), r.getAssetHistory)
			}
		}

		api.GET("/health", r.healthCheck)
	}
}

func (r *RouterController) Run() {
	addr := fmt.Sprintf("%v:%v", r.config.ApiBindAddress, r.config.ApiPort)

	common.Infof("HTTP API available at: %v\n", addr)
	r.engine.Run(addr)
}
