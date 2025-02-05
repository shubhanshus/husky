package otlp

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc/metadata"
)

func TestParseGrpcMetadataIntoRequestInfo(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		apiKeyHeader:    "test-api-key",
		datasetHeader:   "test-dataset",
		userAgentHeader: "test-user-agent",
	}))
	ri := GetRequestInfoFromGrpcMetadata(ctx)

	assert.Equal(t, "test-api-key", ri.ApiKey)
	assert.Equal(t, "test-dataset", ri.Dataset)
	assert.Equal(t, "test-user-agent", ri.UserAgent)
	assert.Equal(t, "application/protobuf", ri.ContentType)
}

func TestParseHttpHeadersIntoRequestInfo(t *testing.T) {
	header := http.Header{}
	header.Set(apiKeyHeader, "test-api-key")
	header.Set(datasetHeader, "test-dataset")
	header.Set(userAgentHeader, "test-user-agent")
	header.Set(contentTypeHeader, "test-content-type")

	ri := GetRequestInfoFromHttpHeaders(header)
	assert.Equal(t, "test-api-key", ri.ApiKey)
	assert.Equal(t, "test-dataset", ri.Dataset)
	assert.Equal(t, "test-user-agent", ri.UserAgent)
	assert.Equal(t, "test-content-type", ri.ContentType)
}

func TestAddAttributesToMap(t *testing.T) {
	testCases := []struct {
		key       string
		expected  interface{}
		attribute *common.KeyValue
	}{
		{
			key:      "str-attr",
			expected: "str-value",
			attribute: &common.KeyValue{
				Key: "str-attr", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "str-value"}},
			},
		},
		{
			key:      "int-attr",
			expected: int64(123),
			attribute: &common.KeyValue{
				Key: "int-attr", Value: &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 123}},
			},
		},
		{
			key:      "double-attr",
			expected: float64(12.3),
			attribute: &common.KeyValue{
				Key: "double-attr", Value: &common.AnyValue{Value: &common.AnyValue_DoubleValue{DoubleValue: 12.3}},
			},
		},
		{
			key:      "bool-attr",
			expected: true,
			attribute: &common.KeyValue{
				Key: "bool-attr", Value: &common.AnyValue{Value: &common.AnyValue_BoolValue{BoolValue: true}},
			},
		},
		{
			key:      "empty-key",
			expected: nil,
			attribute: &common.KeyValue{
				Key: "", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "str-value"}},
			},
		},
		{
			key:      "array-attr",
			expected: "[\"one\",true,3]\n",
			attribute: &common.KeyValue{
				Key: "array-attr", Value: &common.AnyValue{Value: &common.AnyValue_ArrayValue{ArrayValue: &common.ArrayValue{
					Values: []*common.AnyValue{
						{Value: &common.AnyValue_StringValue{StringValue: "one"}},
						{Value: &common.AnyValue_BoolValue{BoolValue: true}},
						{Value: &common.AnyValue_IntValue{IntValue: 3}},
					}}}},
			},
		},
		// Testing single-layer maps is valid but may fail due to map iteration order differences, and
		// that functionality is more completely tested by Test_getValue(). The case of a nested map will fail
		// badly in the way this test is structured, so we don't do maps at all here.
		{
			key:       "nil-value-attr",
			expected:  nil,
			attribute: &common.KeyValue{Key: "kv-attr", Value: nil},
		},
	}

	for _, tc := range testCases {
		attrs := map[string]interface{}{}
		addAttributesToMap(attrs, []*common.KeyValue{tc.attribute})
		assert.Equal(t, tc.expected, attrs[tc.key])
	}
}

func TestValidateTracesHeaders(t *testing.T) {
	testCases := []struct {
		name        string
		apikey      string
		dataset     string
		contentType string
		err         error
	}{
		{name: "no key, no dataset", apikey: "", dataset: "", contentType: "", err: ErrMissingAPIKeyHeader},
		{name: "no key, dataset present", apikey: "", dataset: "dataset", contentType: "", err: ErrMissingAPIKeyHeader},
		{name: "classic/no dataset", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "", contentType: "", err: ErrMissingDatasetHeader},
		{name: "classic/dataset present", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "E&S/no dataset", apikey: "abc123DEF456ghi789jklm", dataset: "", contentType: "application/protobuf", err: nil},
		{name: "E&S/dataset present", apikey: "abc123DEF456ghi789jklm", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/(missing)", apikey: "apikey", dataset: "dataset", contentType: "", err: ErrInvalidContentType},
		{name: "content-type/javascript", apikey: "apikey", dataset: "dataset", contentType: "application/javascript", err: ErrInvalidContentType},
		{name: "content-type/xml", apikey: "apikey", dataset: "dataset", contentType: "application/xml", err: ErrInvalidContentType},
		{name: "content-type/octet-stream", apikey: "apikey", dataset: "dataset", contentType: "application/octet-stream", err: ErrInvalidContentType},
		{name: "content-type/text-plain", apikey: "apikey", dataset: "dataset", contentType: "text-plain", err: ErrInvalidContentType},
		{name: "content-type/json", apikey: "apikey", dataset: "dataset", contentType: "application/json", err: nil},
		{name: "content-type/protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/x-protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/x-protobuf", err: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ri := RequestInfo{ApiKey: tc.apikey, ContentType: tc.contentType, Dataset: tc.dataset}
			err := ri.ValidateTracesHeaders()
			if tc.err != nil {
				assert.EqualError(t, tc.err, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMetricsHeaders(t *testing.T) {
	testCases := []struct {
		name        string
		apikey      string
		dataset     string
		contentType string
		err         error
	}{
		{name: "no key, no dataset", apikey: "", dataset: "", contentType: "", err: ErrMissingAPIKeyHeader},
		{name: "no key, dataset present", apikey: "", dataset: "dataset", contentType: "", err: ErrMissingAPIKeyHeader},
		// classic environments need to tell us which dataset to put metrics in
		{name: "classic/no dataset", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "", contentType: "", err: ErrMissingDatasetHeader},
		{name: "classic/dataset present", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "dataset", contentType: "application/protobuf", err: nil},
		// dataset header not required for E&S, there's a fallback
		{name: "E&S/no dataset", apikey: "abc123DEF456ghi789jklm", dataset: "", contentType: "application/protobuf", err: nil},
		{name: "E&S/dataset present", apikey: "abc123DEF456ghi789jklm", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/(missing)", apikey: "apikey", dataset: "dataset", contentType: "", err: ErrInvalidContentType},
		{name: "content-type/javascript", apikey: "apikey", dataset: "dataset", contentType: "application/javascript", err: ErrInvalidContentType},
		{name: "content-type/xml", apikey: "apikey", dataset: "dataset", contentType: "application/xml", err: ErrInvalidContentType},
		{name: "content-type/octet-stream", apikey: "apikey", dataset: "dataset", contentType: "application/octet-stream", err: ErrInvalidContentType},
		{name: "content-type/text-plain", apikey: "apikey", dataset: "dataset", contentType: "text-plain", err: ErrInvalidContentType},
		{name: "content-type/json", apikey: "apikey", dataset: "dataset", contentType: "application/json", err: nil},
		{name: "content-type/protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/x-protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/x-protobuf", err: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ri := RequestInfo{ApiKey: tc.apikey, ContentType: tc.contentType, Dataset: tc.dataset}
			err := ri.ValidateMetricsHeaders()
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogsHeaders(t *testing.T) {
	testCases := []struct {
		name        string
		apikey      string
		dataset     string
		contentType string
		err         error
	}{
		{name: "no key, no dataset", apikey: "", dataset: "", contentType: "", err: ErrMissingAPIKeyHeader},
		{name: "no key, dataset present", apikey: "", dataset: "dataset", contentType: "", err: ErrMissingAPIKeyHeader},
		// logs will use dataset header if present, but log ingest will also use service.name in the data
		// and we will have a sensible default if neither are present, so a missing dataset header is not an error here
		{name: "classic/no dataset", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "", contentType: "application/protobuf", err: nil},
		{name: "classic/dataset present", apikey: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "E&S/no dataset", apikey: "abc123DEF456ghi789jklm", dataset: "", contentType: "application/protobuf", err: nil},
		{name: "E&S/dataset present", apikey: "abc123DEF456ghi789jklm", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/(missing)", apikey: "apikey", dataset: "dataset", contentType: "", err: ErrInvalidContentType},
		{name: "content-type/javascript", apikey: "apikey", dataset: "dataset", contentType: "application/javascript", err: ErrInvalidContentType},
		{name: "content-type/xml", apikey: "apikey", dataset: "dataset", contentType: "application/xml", err: ErrInvalidContentType},
		{name: "content-type/octet-stream", apikey: "apikey", dataset: "dataset", contentType: "application/octet-stream", err: ErrInvalidContentType},
		{name: "content-type/text-plain", apikey: "apikey", dataset: "dataset", contentType: "text-plain", err: ErrInvalidContentType},
		{name: "content-type/json", apikey: "apikey", dataset: "dataset", contentType: "application/json", err: nil},
		{name: "content-type/protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/protobuf", err: nil},
		{name: "content-type/x-protobuf", apikey: "apikey", dataset: "dataset", contentType: "application/x-protobuf", err: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ri := RequestInfo{ApiKey: tc.apikey, ContentType: tc.contentType, Dataset: tc.dataset}
			err := ri.ValidateLogsHeaders()
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetRequestInfoFromGrpcMetadataIsCaseInsensitive(t *testing.T) {
	const (
		apiKeyValue     = "test-apikey"
		datasetValue    = "test-dataset"
		proxyTokenValue = "test-token"
	)

	tests := []struct {
		name             string
		apikeyHeader     string
		datasetHeader    string
		proxyTokenHeader string
	}{
		{
			name:             "lowercase",
			apikeyHeader:     "x-honeycomb-team",
			datasetHeader:    "x-honeycomb-dataset",
			proxyTokenHeader: "x-honeycomb-proxy-token",
		},
		{
			name:             "uppercase",
			apikeyHeader:     "X-HONEYCOMB-TEAM",
			datasetHeader:    "X-HONEYCOMB-DATASET",
			proxyTokenHeader: "X-HONEYCOMB-PROXY-TOKEN",
		},
		{
			name:             "mixed-case",
			apikeyHeader:     "x-HoNeYcOmB-tEaM",
			datasetHeader:    "X-hOnEyCoMb-DaTaSeT",
			proxyTokenHeader: "X-hOnEyCoMb-PrOxY-tOKeN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := metadata.MD{}
			md.Set(tt.apikeyHeader, apiKeyValue)
			md.Set(tt.datasetHeader, datasetValue)

			ctx := metadata.NewIncomingContext(context.Background(), md)
			ri := GetRequestInfoFromGrpcMetadata(ctx)
			assert.Equal(t, apiKeyValue, ri.ApiKey)
			assert.Equal(t, datasetValue, ri.Dataset)
		})
	}
}

func TestGetRequestInfoFromHttpHeadersIsCaseInsensitive(t *testing.T) {
	const (
		apiKeyValue     = "test-apikey"
		datasetValue    = "test-dataset"
		proxyTokenValue = "test-token"
	)

	tests := []struct {
		name             string
		apikeyHeader     string
		datasetHeader    string
		proxyTokenHeader string
	}{
		{
			name:             "lowercase",
			apikeyHeader:     "x-honeycomb-team",
			datasetHeader:    "x-honeycomb-dataset",
			proxyTokenHeader: "x-honeycomb-proxy-token",
		},
		{
			name:             "uppercase",
			apikeyHeader:     "X-HONEYCOMB-TEAM",
			datasetHeader:    "X-HONEYCOMB-DATASET",
			proxyTokenHeader: "X-HONEYCOMB-PROXY-TOKEN",
		},
		{
			name:             "mixed-case",
			apikeyHeader:     "x-HoNeYcOmB-tEaM",
			datasetHeader:    "X-hOnEyCoMb-DaTaSeT",
			proxyTokenHeader: "X-hOnEyCoMb-PrOxY-tOKeN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set(apiKeyHeader, apiKeyValue)
			header.Set(datasetHeader, datasetValue)

			ri := GetRequestInfoFromHttpHeaders(header)
			assert.Equal(t, apiKeyValue, ri.ApiKey)
			assert.Equal(t, datasetValue, ri.Dataset)
		})
	}
}

func Test_getValue(t *testing.T) {
	tests := []struct {
		name  string
		value *common.AnyValue
		want  interface{}
	}{
		{"int64", &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 123}}, int64(123)},
		{"bool", &common.AnyValue{Value: &common.AnyValue_BoolValue{BoolValue: true}}, true},
		{"float64", &common.AnyValue{Value: &common.AnyValue_DoubleValue{DoubleValue: 123}}, float64(123)},
		{"bytes as b64", &common.AnyValue{Value: &common.AnyValue_BytesValue{BytesValue: []byte{10, 20, 30}}}, `"ChQe"` + "\n"},
		{"array as mixed-type string", &common.AnyValue{Value: &common.AnyValue_ArrayValue{
			ArrayValue: &common.ArrayValue{Values: []*common.AnyValue{
				{Value: &common.AnyValue_IntValue{IntValue: 123}},
				{Value: &common.AnyValue_DoubleValue{DoubleValue: 45.6}},
				{Value: &common.AnyValue_StringValue{StringValue: "hi mom"}},
			}},
		}}, `[123,45.6,"hi mom"]` + "\n"},
		{"map as mixed-type string", &common.AnyValue{
			Value: &common.AnyValue_KvlistValue{KvlistValue: &common.KeyValueList{
				Values: []*common.KeyValue{
					{Key: "foo", Value: &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 123}}},
					{Key: "bar", Value: &common.AnyValue{Value: &common.AnyValue_DoubleValue{DoubleValue: 45.6}}},
					{Key: "mom", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "hi mom"}}},
				},
			}}}, `{"foo":123,"bar":45.6,"mom":"hi mom"}` + "\n"},
		{"nested map as mixed-type string", &common.AnyValue{
			Value: &common.AnyValue_KvlistValue{KvlistValue: &common.KeyValueList{
				Values: []*common.KeyValue{
					{Key: "foo", Value: &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 123}}},
					{Key: "bar", Value: &common.AnyValue{Value: &common.AnyValue_DoubleValue{DoubleValue: 45.6}}},
					{Key: "nest", Value: &common.AnyValue{
						Value: &common.AnyValue_KvlistValue{KvlistValue: &common.KeyValueList{
							Values: []*common.KeyValue{
								{Key: "foo", Value: &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 123}}},
								{Key: "bar", Value: &common.AnyValue{Value: &common.AnyValue_DoubleValue{DoubleValue: 45.6}}},
								{Key: "mom", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "hi mom"}}},
							},
						}}}},
				},
			}}}, `{"bar":45.6,"foo":123,"nest":{"bar":45.6,"foo":123,"mom":"hi mom"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := getValue(tt.value)
			if truncated != 0 {
				t.Errorf("getValue() returned %v for truncatedBytes, should be 0", truncated)
			}
			if s, ok := got.(string); ok && strings.HasPrefix(s, "{") {
				// it's a string wrapping an object, and might be out of order, so convert them both to objects
				// and compare them as unmarshalled objects
				var g, w map[string]any
				json.Unmarshal([]byte(s), &g)
				json.Unmarshal([]byte(tt.want.(string)), &w)
				if !reflect.DeepEqual(g, w) {
					t.Errorf("getValue() unmarshalled = %#v, want %#v", g, w)
					t.Errorf("getValue() marshalled = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
				}
			} else {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("getValue() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
				}
			}
		})
	}
}

func Test_limitedWriter(t *testing.T) {
	tests := []struct {
		name      string
		max       int
		input     []string
		total     int
		want      string
		wantTrunc int
	}{
		{"no limit", 100, []string{"abcde"}, 5, "abcde", 0},
		{"one write", 5, []string{"abcdefghij"}, 10, "abcde", 5},
		{"two writes", 12, []string{"abcdefghij", "abcdefghij"}, 20, "abcdefghijab", 8},
		{"exact overrun", 10, []string{"abcdefghij", "abcdefghij"}, 20, "abcdefghij", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLimitedWriter(tt.max)
			total := 0
			for _, s := range tt.input {
				n, err := l.Write([]byte(s))
				if err != nil {
					t.Errorf("limitedWriter.Write() error = %v", err)
					return
				}
				total += n
			}
			if total != tt.total {
				t.Errorf("limitedWriter.Write() total was %v, want %v", total, tt.total)
			}
			s := l.String()
			if s != tt.want {
				t.Errorf("limitedWriter.String() = '%v', want '%v'", s, tt.want)
			}
		})
	}
}
