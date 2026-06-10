package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"axis-flow-back/internal/asignacion"
	"axis-flow-back/internal/asignacion/storage"
)

type fakeS3Client struct {
	putObjectCalls []s3.PutObjectInput
	putObjectErr   error
}

func (f *fakeS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putObjectCalls = append(f.putObjectCalls, *params)
	if f.putObjectErr != nil {
		return nil, f.putObjectErr
	}
	return &s3.PutObjectOutput{}, nil
}

func readPutObjectBody(t *testing.T, body io.Reader) []byte {
	t.Helper()
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	return raw
}

func TestS3EvidenceUploaderUploadsBytesAndReturnsCanonicalURL(t *testing.T) {
	fake := &fakeS3Client{}
	activityID := uuid.MustParse("d8a85f64-5717-4562-b3fc-2c963f66afc0")
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{Bucket: "checkon-evidences"})

	payload := []byte("jpgdata")
	url, err := uploader.Upload(context.Background(), activityID, 1, bytes.NewReader(payload), "image/jpeg", "evidencia_1.jpg")
	require.NoError(t, err)

	require.Len(t, fake.putObjectCalls, 1)
	call := fake.putObjectCalls[0]
	assert.Equal(t, "checkon-evidences", *call.Bucket)
	require.NotNil(t, call.Key)
	assert.Equal(t, "evidencia/1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg", *call.Key)
	assert.Equal(t, "image/jpeg", *call.ContentType)
	assert.Equal(t, payload, readPutObjectBody(t, call.Body))
	assert.Equal(t, "https://checkon-evidences.s3.amazonaws.com/evidencia/1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg", url)
}

func TestS3EvidenceUploaderRejectsOversizedContent(t *testing.T) {
	fake := &fakeS3Client{}
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{
		Bucket:         "checkon-evidences",
		MaxObjectBytes: 1024,
	})

	oversized := bytes.Repeat([]byte{0x42}, 2048)
	_, err := uploader.Upload(context.Background(), uuid.New(), 1, bytes.NewReader(oversized), "image/jpeg", "evidencia_1.jpg")
	require.Error(t, err)
	require.ErrorIs(t, err, asignacion.ErrInvalidEvidenceFile)
	assert.Empty(t, fake.putObjectCalls, "S3 must not be called when the payload exceeds the size cap")
}

func TestS3EvidenceUploaderMapsPutObjectError(t *testing.T) {
	fake := &fakeS3Client{putObjectErr: errors.New("aws boom")}
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{Bucket: "checkon-evidences"})

	_, err := uploader.Upload(context.Background(), uuid.New(), 2, bytes.NewReader([]byte("pngdata")), "image/png", "evidencia_2.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aws boom")
}

func TestS3EvidenceUploaderResolvesExtensionFromFilename(t *testing.T) {
	cases := []struct {
		filename string
		wantExt  string
	}{
		{"evidencia_1.png", "png"},
		{"evidencia_1.PNG", "png"},
		{"photo.jpeg", "jpg"},
		{"photo.JPG", "jpg"},
		{"mystery", "jpg"},
	}
	for _, tc := range cases {
		fake := &fakeS3Client{}
		uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{Bucket: "checkon-evidences"})
		_, err := uploader.Upload(context.Background(), uuid.New(), 1, bytes.NewReader([]byte("x")), "image/jpeg", tc.filename)
		require.NoError(t, err)
		require.Len(t, fake.putObjectCalls, 1)
		key := *fake.putObjectCalls[0].Key
		assert.Contains(t, key, "."+tc.wantExt, "filename=%q", tc.filename)
	}
}

func TestS3EvidenceUploaderAppliesDefaults(t *testing.T) {
	fake := &fakeS3Client{}
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{Bucket: "checkon-evidences"})

	_, err := uploader.Upload(context.Background(), uuid.New(), 1, bytes.NewReader([]byte("x")), "image/jpeg", "evidencia_1.jpg")
	require.NoError(t, err)
	require.Len(t, fake.putObjectCalls, 1)
	assert.Equal(t, "evidencia/", (*fake.putObjectCalls[0].Key)[:10])
}

func TestS3EvidenceUploaderHonorsCustomPublicBaseURL(t *testing.T) {
	fake := &fakeS3Client{}
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{
		Bucket:        "checkon-evidences",
		PublicBaseURL: "https://cdn.example.com",
	})
	url, err := uploader.Upload(context.Background(), uuid.New(), 1, bytes.NewReader([]byte("x")), "image/jpeg", "evidencia_1.jpg")
	require.NoError(t, err)
	assert.True(t, len(url) > len("https://cdn.example.com/"))
	assert.Contains(t, url, "https://cdn.example.com/")
}

func TestS3EvidenceUploaderReadBytesDoesNotExceedCap(t *testing.T) {
	fake := &fakeS3Client{}
	uploader := storage.NewS3EvidenceUploader(fake, storage.S3EvidenceUploaderConfig{
		Bucket:         "checkon-evidences",
		MaxObjectBytes: 4,
	})
	_, err := uploader.Upload(context.Background(), uuid.New(), 1, bytes.NewReader([]byte("0123456789")), "image/jpeg", "evidencia_1.jpg")
	require.Error(t, err)
	assert.Empty(t, fake.putObjectCalls)
}
