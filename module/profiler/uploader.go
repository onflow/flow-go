package profiler

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"google.golang.org/api/option"
	gtransport "google.golang.org/api/transport/grpc"
	pb "google.golang.org/genproto/googleapis/devtools/cloudprofiler/v2"
	"google.golang.org/grpc"
)

const (
	apiAddress = "cloudprofiler.googleapis.com:443"
	scope      = "https://www.googleapis.com/auth/monitoring.write"

	maxMsgSize = 1 << 30 // 1GB
)

type NoopUploader struct{}

func (u *NoopUploader) Upload(ctx context.Context, filename string, pt pb.ProfileType) error {
	return nil
}

type Uploader interface {
	Upload(ctx context.Context, filename string, pt pb.ProfileType) error
}

type Params struct {
	ProjectID string
	ChainID   string
	Role      string
	Version   string
	Commit    string
	Instance  string
}

type UploaderImpl struct {
	log    zerolog.Logger
	client pb.ProfilerServiceClient

	ProjectId  string
	Deployment *pb.Deployment
}

func NewUploader(log zerolog.Logger, params Params, opts ...option.ClientOption) (Uploader, error) {
	log = log.With().Str("component", "profile_uploader").Logger()

	defaultOpts := []option.ClientOption{
		option.WithEndpoint(apiAddress),
		option.WithScopes(scope),
		option.WithUserAgent("OfflinePprofUploader"),
		option.WithTelemetryDisabled(),
		option.WithGRPCDialOption(
			grpc.WithDefaultCallOptions(
				grpc.MaxCallSendMsgSize(maxMsgSize),
			),
		),
	}
	opts = append(defaultOpts, opts...)

	connPool, err := gtransport.DialPool(
		context.Background(),
		opts...,
	)
	if err != nil {
		return &NoopUploader{}, fmt.Errorf("failed to create connection pool: %w", err)
	}

	version := fmt.Sprintf("%s-%s", params.Version, params.Commit)
	targetName := fmt.Sprintf("%s-%s", params.ChainID, params.Role)
	deployment := &pb.Deployment{
		ProjectId: params.ProjectID,
		Target:    targetName,
		Labels: map[string]string{
			"language": "go",
			"version":  version,
			"instance": params.Instance,
		},
	}

	u := &UploaderImpl{
		log:    log,
		client: pb.NewProfilerServiceClient(connPool),

		ProjectId:  params.ProjectID,
		Deployment: deployment,
	}

	return u, nil
}

func (u *UploaderImpl) Upload(ctx context.Context, filename string, pt pb.ProfileType) error {
	profileBytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read profile: %w", err)
	}

	req := pb.CreateOfflineProfileRequest{
		Parent: u.ProjectId,
		Profile: &pb.Profile{
			ProfileType:  pt,
			Deployment:   u.Deployment,
			ProfileBytes: profileBytes,
		},
	}

	resp, err := u.client.CreateOfflineProfile(ctx, &req)
	if err != nil {
		return fmt.Errorf("failed to create offline profile: %w", err)
	}
	u.log.Info().
		Str("file_name", filename).
		Str("profile_name", resp.GetName()).
		Int("profile_size", len(profileBytes)).
		Msg("uploaded profile")
	return nil
}
