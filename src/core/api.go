package core

import (
	"log/slog"
)

// api layer to be accessed through websockets, http and other exposed apis
//
//	[Layer 1: Exposed APIs]
//	+-----------------+     +----------------+
//	|  WebsocketAPI   |     |     HttpAPI     |
//	|-----------------|     |-----------------|
//	| - Parse Inputs  |     | - Parse Inputs  |
//	| - Serialize Data|     | - Serialize Data|
//	+-----------------+     +-----------------+
//	        |                     |
//	        \_____________________/
//	                  |
//	                  V
//	[Layer 2: API Interface]
//	+---------------------------------+
//	|         API Interface           |
//	|---------------------------------|
//	| - Unified High-Level API Calls  |
//	+---------------------------------+
//	                  |
//	                  V
//	[Layer 3: Services]
//	+--------------------+   +--------------------+   +--------------------+
//	|     Service 1      |   |     Service 2      |   |     Service N      |
//	|--------------------|   |--------------------|   |--------------------|
//	| - Manages Subsystem|   | - Manages Subsystem|   | - Manages Subsystem|
//	+--------------------+   +--------------------+   +--------------------+
//	                  |
//	                  V
//	[Layer 4: Packages/Modules]
//	+-------------------+   +-------------------+   +-------------------+
//	|   Package/Mod1    |   |   Package/Mod2    |   |   Package/ModN    |
//	|-------------------|   |-------------------|   |-------------------|
//	| - Low-Level Ops   |   | - Low-Level Ops   |   | - Low-Level Ops   |
//	+-------------------+   +-------------------+   +-------------------+
type Api interface {
	WorkspaceListResources() // TODO: just a draft, this needs args
}

type api struct {
	workspaceManager WorkspaceManager
	logger           *slog.Logger
}

func NewApi(logger *slog.Logger, workspaceManager WorkspaceManager) Api {
	apiModule := &api{}

	apiModule.logger = logger
	apiModule.workspaceManager = workspaceManager

	return apiModule
}

func (self *api) WorkspaceListResources() {}
