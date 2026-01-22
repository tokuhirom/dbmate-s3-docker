package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// MockS3Client is an in-memory mock implementation of S3 client for unit tests
type MockS3Client struct {
	mu      sync.RWMutex
	objects map[string][]byte // key -> content
}

// NewMockS3Client creates a new mock S3 client
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		objects: make(map[string][]byte),
	}
}

// PutObject stores an object in the mock storage
func (m *MockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if input.Bucket == nil || input.Key == nil {
		return nil, fmt.Errorf("bucket and key are required")
	}

	// Read the body content
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}

	key := *input.Bucket + "/" + *input.Key
	m.objects[key] = content

	return &s3.PutObjectOutput{}, nil
}

// GetObject retrieves an object from the mock storage
func (m *MockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if input.Bucket == nil || input.Key == nil {
		return nil, fmt.Errorf("bucket and key are required")
	}

	key := *input.Bucket + "/" + *input.Key
	content, exists := m.objects[key]
	if !exists {
		return nil, &types.NoSuchKey{
			Message: aws.String("The specified key does not exist"),
		}
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(content)),
	}, nil
}

// HeadObject checks if an object exists in the mock storage
func (m *MockS3Client) HeadObject(ctx context.Context, input *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if input.Bucket == nil || input.Key == nil {
		return nil, fmt.Errorf("bucket and key are required")
	}

	key := *input.Bucket + "/" + *input.Key
	_, exists := m.objects[key]
	if !exists {
		return nil, &types.NotFound{
			Message: aws.String("Not Found"),
		}
	}

	return &s3.HeadObjectOutput{}, nil
}

// ListObjectsV2 lists objects with a given prefix in the mock storage
func (m *MockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if input.Bucket == nil {
		return nil, fmt.Errorf("bucket is required")
	}

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	delimiter := ""
	if input.Delimiter != nil {
		delimiter = *input.Delimiter
	}

	bucketPrefix := *input.Bucket + "/"
	var contents []types.Object
	commonPrefixes := make(map[string]bool)

	for key, content := range m.objects {
		// Check if key belongs to this bucket
		if !strings.HasPrefix(key, bucketPrefix) {
			continue
		}

		// Remove bucket prefix
		objectKey := strings.TrimPrefix(key, bucketPrefix)

		// Check if key matches prefix
		if prefix != "" && !strings.HasPrefix(objectKey, prefix) {
			continue
		}

		// Handle delimiter for directory-like listing
		if delimiter != "" {
			// Remove the prefix from key
			remaining := strings.TrimPrefix(objectKey, prefix)

			// Check if there's a delimiter in the remaining path
			delimiterIndex := strings.Index(remaining, delimiter)
			if delimiterIndex >= 0 {
				// This is a "directory" - add to common prefixes
				commonPrefix := prefix + remaining[:delimiterIndex+1]
				commonPrefixes[commonPrefix] = true
				continue
			}
		}

		// Add to contents
		contents = append(contents, types.Object{
			Key:  aws.String(objectKey),
			Size: aws.Int64(int64(len(content))),
		})
	}

	// Convert common prefixes map to slice
	var commonPrefixesSlice []types.CommonPrefix
	for cp := range commonPrefixes {
		commonPrefixesSlice = append(commonPrefixesSlice, types.CommonPrefix{
			Prefix: aws.String(cp),
		})
	}

	return &s3.ListObjectsV2Output{
		Contents:       contents,
		CommonPrefixes: commonPrefixesSlice,
	}, nil
}

// DeleteObject removes an object from the mock storage
func (m *MockS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if input.Bucket == nil || input.Key == nil {
		return nil, fmt.Errorf("bucket and key are required")
	}

	key := *input.Bucket + "/" + *input.Key
	delete(m.objects, key)

	return &s3.DeleteObjectOutput{}, nil
}

// Clear removes all objects from the mock storage
func (m *MockS3Client) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects = make(map[string][]byte)
}

// ObjectCount returns the number of objects in the mock storage
func (m *MockS3Client) ObjectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// HasObject checks if a specific object exists
func (m *MockS3Client) HasObject(bucket, key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fullKey := bucket + "/" + key
	_, exists := m.objects[fullKey]
	return exists
}

// GetObjectContent retrieves the content of an object as a string
func (m *MockS3Client) GetObjectContent(bucket, key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fullKey := bucket + "/" + key
	content, exists := m.objects[fullKey]
	if !exists {
		return "", false
	}
	return string(content), true
}
