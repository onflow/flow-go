package profiler

import (
	"context"
	"os"
	"testing"

	"cloud.google.com/go/profiler/mocks"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	pb "google.golang.org/genproto/googleapis/devtools/cloudprofiler/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestProfilerUpload(t *testing.T) {
	params := Params{
		ProjectID: "project_id",
		ChainID:   "chain_id",
		Role:      "role",
		Version:   "version",
		Commit:    "commit",
		Instance:  "instance",
	}
	uploader, err := NewUploader(
		zerolog.Nop(),
		params,

		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	require.NoError(t, err)

	uploaderImpl, ok := uploader.(*UploaderImpl)
	require.True(t, ok)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mocks.NewMockProfilerServiceClient(ctrl)
	uploaderImpl.client = mock

	mock.EXPECT().CreateOfflineProfile(gomock.Any(), gomock.Any()).
		Return(&pb.Profile{Name: "test"}, nil).
		Times(1).
		Do(func(ctx context.Context, req *pb.CreateOfflineProfileRequest, opts ...grpc.CallOption) {
			require.Equal(t, "project_id", req.Parent)
			require.Equal(t, "project_id", req.Profile.Deployment.GetProjectId())
			require.Equal(t, "chain_id-role", req.Profile.Deployment.GetTarget())
			require.Equal(t, "version-commit", req.Profile.Deployment.GetLabels()["version"])
		})

	f, err := os.CreateTemp("", "example")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	err = uploader.Upload(context.Background(), f.Name(), pb.ProfileType_CPU)
	require.NoError(t, err)
}
