package minio

import (
	"context"
	"fmt"

	actions "github.com/exyb/safe-input/client/cmd"
	"github.com/minio/minio-go/v7"
)

func HandleUndoCommand(ctx context.Context, client *minio.Client, lastAction *actions.LastAction) error {
	switch lastAction.Action {
	case "rm":
		fmt.Println("无法撤销删除操作")
	case "mv":
		// 实现从 lastAction.object 还原的逻辑
	default:
		fmt.Println("没有可撤销的操作")
	}
	return nil
}
