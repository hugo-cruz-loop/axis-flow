// Package service implements business orchestration for the empleados module.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

const defaultFaceSimilarityThreshold = 70.0

// ErrRekognitionDisabled is returned when biometric comparison is disabled by configuration.
var ErrRekognitionDisabled = errors.New("rekognition is disabled")

// FaceComparisonResult is the domain-level result of a facial comparison.
type FaceComparisonResult struct {
	Similarity float64
	Passed     bool
}

// FaceComparator is the service port used by biometric workflows.
type FaceComparator interface {
	CompareFaces(ctx context.Context, sourceImage, targetImage []byte) (FaceComparisonResult, error)
}

// CompareFacesAPI is the minimal AWS Rekognition SDK boundary needed by RekognitionClient.
type CompareFacesAPI interface {
	CompareFaces(ctx context.Context, input *rekognition.CompareFacesInput, optFns ...func(*rekognition.Options)) (*rekognition.CompareFacesOutput, error)
}

// RekognitionClient compares employee face images through AWS Rekognition.
type RekognitionClient struct {
	api       CompareFacesAPI
	enabled   bool
	threshold float64
}

// NewRekognitionClient creates a testable Rekognition client over the AWS SDK CompareFaces API.
func NewRekognitionClient(api CompareFacesAPI, enabled bool, threshold float64) *RekognitionClient {
	if threshold <= 0 {
		threshold = defaultFaceSimilarityThreshold
	}
	return &RekognitionClient{api: api, enabled: enabled, threshold: threshold}
}

// CompareFaces compares source and target image bytes and evaluates the configured similarity threshold.
func (c *RekognitionClient) CompareFaces(ctx context.Context, sourceImage, targetImage []byte) (FaceComparisonResult, error) {
	if !c.enabled {
		return FaceComparisonResult{}, ErrRekognitionDisabled
	}
	if c.api == nil {
		return FaceComparisonResult{}, errors.New("rekognition API client is required")
	}

	out, err := c.api.CompareFaces(ctx, &rekognition.CompareFacesInput{
		SourceImage:         &types.Image{Bytes: sourceImage},
		TargetImage:         &types.Image{Bytes: targetImage},
		SimilarityThreshold: aws.Float32(float32(c.threshold)),
	})
	if err != nil {
		return FaceComparisonResult{}, fmt.Errorf("rekognition CompareFaces: %w", err)
	}

	similarity := highestSimilarity(out.FaceMatches)
	return FaceComparisonResult{Similarity: similarity, Passed: similarity >= c.threshold}, nil
}

func highestSimilarity(matches []types.CompareFacesMatch) float64 {
	var highest float64
	for _, match := range matches {
		if match.Similarity == nil {
			continue
		}
		value := float64(*match.Similarity)
		if value > highest {
			highest = value
		}
	}
	return highest
}
