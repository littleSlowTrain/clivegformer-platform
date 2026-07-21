package initialize

import (
	"context"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func Ceph(endpoint, accessKey, secretKey, region string, secure bool) (*minio.Client, *minio.Core, error) {
	opts := &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure, Region: region, BucketLookup: minio.BucketLookupPath}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, nil, err
	}
	core, err := minio.NewCore(endpoint, opts)
	if err != nil {
		return nil, nil, err
	}
	return client, core, nil
}

type EventProducer struct{ client rocketmq.Producer }

func NewEventProducer(namesrv string) (*EventProducer, error) {
	p, err := rocketmq.NewProducer(producer.WithNameServer([]string{namesrv}), producer.WithRetry(2))
	if err != nil {
		return nil, err
	}
	if err := p.Start(); err != nil {
		return nil, err
	}
	return &EventProducer{client: p}, nil
}
func (p *EventProducer) Send(body []byte) error {
	_, err := p.client.SendSync(context.Background(), primitive.NewMessage("FILE_UPLOAD_COMPLETE", body))
	return err
}
func (p *EventProducer) Shutdown() error { return p.client.Shutdown() }
