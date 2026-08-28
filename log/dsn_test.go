package log

import (
	"testing"
	"time"
)

func TestParseFileOptions(t *testing.T) {
	tests := []struct {
		want    *FileOptions
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name: "basic file DSN",
			dsn:  "file:///var/log/app.log",
			want: &FileOptions{
				Path:       "/var/log/app.log",
				MaxSize:    100,
				MaxBackups: 10,
				MaxAge:     30,
				Compress:   "none",
			},
			wantErr: false,
		},
		{
			name: "file DSN with all params",
			dsn:  "file:///var/log/app.log?max-size=50m&max-backups=5&max-age=7d&compress=gzip",
			want: &FileOptions{
				Path:       "/var/log/app.log",
				MaxSize:    50,
				MaxBackups: 5,
				MaxAge:     7,
				Compress:   "gzip",
			},
			wantErr: false,
		},
		{
			name: "file DSN with GB size",
			dsn:  "file:///var/log/app.log?max-size=1g",
			want: &FileOptions{
				Path:       "/var/log/app.log",
				MaxSize:    1024,
				MaxBackups: 10,
				MaxAge:     30,
				Compress:   "none",
			},
			wantErr: false,
		},
		{
			name:    "invalid scheme",
			dsn:     "http://localhost/log",
			wantErr: true,
		},
		{
			name:    "invalid max-size",
			dsn:     "file:///var/log/app.log?max-size=invalid",
			wantErr: true,
		},
		{
			name:    "invalid compress format",
			dsn:     "file:///var/log/app.log?compress=zip",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFileOptions(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFileOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path = %v, want %v", got.Path, tt.want.Path)
			}
			if got.MaxSize != tt.want.MaxSize {
				t.Errorf("MaxSize = %v, want %v", got.MaxSize, tt.want.MaxSize)
			}
			if got.MaxBackups != tt.want.MaxBackups {
				t.Errorf("MaxBackups = %v, want %v", got.MaxBackups, tt.want.MaxBackups)
			}
			if got.MaxAge != tt.want.MaxAge {
				t.Errorf("MaxAge = %v, want %v", got.MaxAge, tt.want.MaxAge)
			}
			if got.Compress != tt.want.Compress {
				t.Errorf("Compress = %v, want %v", got.Compress, tt.want.Compress)
			}
		})
	}
}

func TestParseHTTPOptions(t *testing.T) {
	tests := []struct {
		want    *HTTPOptions
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name: "basic HTTP DSN",
			dsn:  "http://localhost:3000/logs",
			want: &HTTPOptions{
				URL:        "http://localhost:3000/logs",
				Timeout:    10 * time.Second,
				BufferSize: 1024,
				BatchSize:  100,
				MaxRetries: 3,
			},
			wantErr: false,
		},
		{
			name: "HTTPS DSN with all params",
			dsn:  "https://logs.example.com/api/v1/logs?timeout=5s&buffer-size=512&batch-size=50&max-retries=5",
			want: &HTTPOptions{
				URL:        "https://logs.example.com/api/v1/logs",
				Timeout:    5 * time.Second,
				BufferSize: 512,
				BatchSize:  50,
				MaxRetries: 5,
			},
			wantErr: false,
		},
		{
			name:    "invalid scheme",
			dsn:     "file:///var/log/app.log",
			wantErr: true,
		},
		{
			name:    "invalid timeout",
			dsn:     "http://localhost:3000/logs?timeout=invalid",
			wantErr: true,
		},
		{
			name:    "invalid buffer-size",
			dsn:     "http://localhost:3000/logs?buffer-size=abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHTTPOptions(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHTTPOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.URL != tt.want.URL {
				t.Errorf("URL = %v, want %v", got.URL, tt.want.URL)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
			if got.BufferSize != tt.want.BufferSize {
				t.Errorf("BufferSize = %v, want %v", got.BufferSize, tt.want.BufferSize)
			}
			if got.BatchSize != tt.want.BatchSize {
				t.Errorf("BatchSize = %v, want %v", got.BatchSize, tt.want.BatchSize)
			}
			if got.MaxRetries != tt.want.MaxRetries {
				t.Errorf("MaxRetries = %v, want %v", got.MaxRetries, tt.want.MaxRetries)
			}
		})
	}
}

func TestParseSizeString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"plain number", "100", 100, false},
		{"megabytes lowercase", "50m", 50, false},
		{"megabytes uppercase", "50M", 50, false},
		{"megabytes with mb", "50mb", 50, false},
		{"gigabytes", "2g", 2048, false},
		{"gigabytes uppercase", "2G", 2048, false},
		{"gigabytes with gb", "2gb", 2048, false},
		{"invalid format", "abc", 0, true},
		{"invalid unit", "100k", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSizeString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSizeString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseSizeString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDurationString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"plain number", "30", 30, false},
		{"days lowercase", "7d", 7, false},
		{"days uppercase", "7D", 7, false},
		{"day singular", "1day", 1, false},
		{"days plural", "30days", 30, false},
		{"invalid format", "abc", 0, true},
		{"invalid unit", "7h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDurationString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDurationString() = %v, want %v", got, tt.want)
			}
		})
	}
}
