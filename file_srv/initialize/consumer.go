package initialize

import (
	"context"
	"encoding/json"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/clivegformer/platform/file_srv/handler"
)

func StartUploadConsumer(namesrv string, server *handler.Server) (rocketmq.PushConsumer, error) {
	c, err := rocketmq.NewPushConsumer(consumer.WithGroupName("file-service-upload-complete"), consumer.WithNameServer([]string{namesrv}))
	if err != nil {
		return nil, err
	}
	err = c.Subscribe("FILE_UPLOAD_COMPLETE", consumer.MessageSelector{}, func(ctx context.Context, messages ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, message := range messages {
			var event handler.UploadCompleteEvent
			if err := json.Unmarshal(message.Body, &event); err != nil {
				return consumer.ConsumeRetryLater, err
			}
			applied, err := server.HandleUploadComplete(ctx, event)
			if err != nil {
				return consumer.ConsumeRetryLater, err
			}
			if applied {
				server.AfterUploadComplete(ctx, event)
			}
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	return c, nil
}
