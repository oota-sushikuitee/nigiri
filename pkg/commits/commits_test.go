package commits

import (
	"testing"
)

func TestCommit_Validate(t *testing.T) {
	tests := []struct {
		name    string
		commit  Commit
		wantErr bool
	}{
		{
			name: "valid commit",
			commit: Commit{
				Hash:      "1234567890abcdef1234567890abcdef12345678",
				ShortHash: "1234567",
			},
			wantErr: false,
		},
		{
			name: "empty hash",
			commit: Commit{
				Hash:      "",
				ShortHash: "1234567",
			},
			wantErr: true,
		},
		{
			name: "hash too short",
			commit: Commit{
				Hash:      "123456",
				ShortHash: "123456",
			},
			wantErr: true,
		},
		{
			name: "empty short hash",
			commit: Commit{
				Hash:      "1234567890abcdef1234567890abcdef12345678",
				ShortHash: "",
			},
			wantErr: true,
		},
		{
			name: "short hash too short",
			commit: Commit{
				Hash:      "1234567890abcdef1234567890abcdef12345678",
				ShortHash: "123456",
			},
			wantErr: true,
		},
		{
			name: "valid short hash only",
			commit: Commit{
				Hash:      "ABCDEF0",
				ShortHash: "ABCDEF0",
			},
			wantErr: false,
		},
		{
			name: "hash with parent directory traversal",
			commit: Commit{
				Hash:      "../../evil",
				ShortHash: "../../e",
			},
			wantErr: true,
		},
		{
			name: "hash with path separator",
			commit: Commit{
				Hash:      "foo/bar/baz",
				ShortHash: "foo/bar",
			},
			wantErr: true,
		},
		{
			name: "hash with non-hex characters",
			commit: Commit{
				Hash:      "zzzzzzzzzz",
				ShortHash: "zzzzzzz",
			},
			wantErr: true,
		},
		{
			name: "hash longer than a full sha-1",
			commit: Commit{
				Hash:      "1234567890abcdef1234567890abcdef123456789",
				ShortHash: "1234567",
			},
			wantErr: true,
		},
		{
			name: "short hash with path separator",
			commit: Commit{
				Hash:      "1234567890abcdef1234567890abcdef12345678",
				ShortHash: "../../e",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.commit.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommit_CalculateShortHash(t *testing.T) {
	tests := []struct {
		name      string
		hash      string
		wantShort string
		wantErr   bool
	}{
		{
			name:      "valid hash",
			hash:      "1234567890abcdef1234567890abcdef12345678",
			wantShort: "1234567",
			wantErr:   false,
		},
		{
			name:      "valid short hash",
			hash:      "abc1234",
			wantShort: "abc1234",
			wantErr:   false,
		},
		{
			name:      "hash too short",
			hash:      "123456",
			wantShort: "",
			wantErr:   true,
		},
		{
			name:      "empty hash",
			hash:      "",
			wantShort: "",
			wantErr:   true,
		},
		{
			name:      "parent directory traversal",
			hash:      "../../evil",
			wantShort: "",
			wantErr:   true,
		},
		{
			name:      "path separator",
			hash:      "foo/bar",
			wantShort: "",
			wantErr:   true,
		},
		{
			name:      "absolute path",
			hash:      "/etc/passwd",
			wantShort: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Commit{
				Hash: tt.hash,
			}
			err := c.CalculateShortHash()
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateShortHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if c.ShortHash != tt.wantShort {
				t.Errorf("CalculateShortHash() got = %v, want %v", c.ShortHash, tt.wantShort)
			}
		})
	}
}
