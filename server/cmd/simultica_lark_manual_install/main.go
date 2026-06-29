package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/integrations/lark"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbURL := mustEnv("DATABASE_URL")
	appID := mustEnv("SIMULTICA_LARK_APP_ID")
	appSecret := mustEnv("SIMULTICA_LARK_APP_SECRET")
	workspaceID := mustUUID("SIMULTICA_WORKSPACE_ID")
	agentID := mustUUID("SIMULTICA_LARK_AGENT_ID")
	installerID := mustUUID("SIMULTICA_LARK_INSTALLER_USER_ID")

	key, err := secretbox.LoadKey("MULTICA_LARK_SECRET_KEY")
	if err != nil {
		log.Fatalf("load lark secret key: %v", err)
	}
	box, err := secretbox.New(key)
	if err != nil {
		log.Fatalf("new secretbox: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	agent, err := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
		ID:          agentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		log.Fatalf("agent not found in workspace: %v", err)
	}

	api := lark.NewHTTPAPIClient(lark.HTTPClientConfig{Logger: slog.Default()})
	botInfo, err := api.GetBotInfo(ctx, lark.InstallationCredentials{
		AppID:     appID,
		AppSecret: appSecret,
		Region:    lark.RegionFeishu,
	})
	if err != nil {
		log.Fatalf("get bot info: %v", err)
	}

	installSvc, err := lark.NewInstallationService(queries, box)
	if err != nil {
		log.Fatalf("new installation service: %v", err)
	}
	inst, err := installSvc.Upsert(ctx, lark.InstallationParams{
		WorkspaceID:     workspaceID,
		AgentID:         agentID,
		AppID:           appID,
		AppSecret:       appSecret,
		BotOpenID:       string(botInfo.OpenID),
		InstallerUserID: installerID,
		Region:          lark.RegionFeishu,
	})
	if err != nil {
		log.Fatalf("upsert installation: %v", err)
	}

	fmt.Printf("installed lark bot\n")
	fmt.Printf("  installation_id: %s\n", util.UUIDToString(inst.ID))
	fmt.Printf("  workspace_id:    %s\n", util.UUIDToString(inst.WorkspaceID))
	fmt.Printf("  agent_id:        %s\n", util.UUIDToString(inst.AgentID))
	fmt.Printf("  agent_name:      %s\n", agent.Name)
	fmt.Printf("  app_id:          %s\n", inst.AppID)
	fmt.Printf("  bot_open_id:     %s\n", inst.BotOpenID)
	if inst.BotUnionID.Valid {
		fmt.Printf("  bot_union_id:    %s\n", inst.BotUnionID.String)
	}
	fmt.Printf("  status:          %s\n", inst.Status)
	fmt.Printf("  region:          %s\n", inst.Region)
}

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func mustUUID(name string) pgtype.UUID {
	value := mustEnv(name)
	u, err := util.ParseUUID(value)
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	return u
}
