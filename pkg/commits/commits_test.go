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
			name:      "hash too short",
			hash:      "123456",
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
			if !tt.wantErr && c.ShortHash != tt.wantShort {
				t.Errorf("CalculateShortHash() got = %v, want %v", c.ShortHash, tt.wantShort)
			}
		})
	}
}
