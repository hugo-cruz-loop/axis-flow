package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/empleados/service"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/stretchr/testify/require"
)

type fakeCompareFacesAPI struct {
	input      *rekognition.CompareFacesInput
	similarity float32
	calls      int
}

func (f *fakeCompareFacesAPI) CompareFaces(ctx context.Context, input *rekognition.CompareFacesInput, opts ...func(*rekognition.Options)) (*rekognition.CompareFacesOutput, error) {
	f.calls++
	f.input = input
	return &rekognition.CompareFacesOutput{
		FaceMatches: []types.CompareFacesMatch{{Similarity: &f.similarity}},
	}, nil
}

func TestRekognitionClientCompareFacesApprovesAboveThreshold(t *testing.T) {
	api := &fakeCompareFacesAPI{similarity: 85.5}
	client := service.NewRekognitionClient(api, true, 70)

	got, err := client.CompareFaces(context.Background(), []byte{0x01, 0x02}, []byte{0x03, 0x04})

	require.NoError(t, err)
	require.True(t, got.Passed)
	require.InDelta(t, 85.5, got.Similarity, 0.001)
	require.Equal(t, float32(70), *api.input.SimilarityThreshold)
	require.Equal(t, []byte{0x01, 0x02}, api.input.SourceImage.Bytes)
	require.Equal(t, []byte{0x03, 0x04}, api.input.TargetImage.Bytes)
}

func TestRekognitionClientCompareFacesRejectsBelowThreshold(t *testing.T) {
	api := &fakeCompareFacesAPI{similarity: 52.4}
	client := service.NewRekognitionClient(api, true, 70)

	got, err := client.CompareFaces(context.Background(), []byte{0x01}, []byte{0x02})

	require.NoError(t, err)
	require.False(t, got.Passed)
	require.InDelta(t, 52.4, got.Similarity, 0.001)
}

func TestRekognitionClientDisabledDoesNotCallAWS(t *testing.T) {
	api := &fakeCompareFacesAPI{similarity: 99}
	client := service.NewRekognitionClient(api, false, 70)

	_, err := client.CompareFaces(context.Background(), []byte{0x01}, []byte{0x02})

	require.ErrorIs(t, err, service.ErrRekognitionDisabled)
	require.Zero(t, api.calls)
}
