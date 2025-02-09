package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "mySecurePassword123!"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Verify hash format
	if !strings.HasPrefix(hash1, "$argon2id$v=") {
		t.Errorf("Hash format is incorrect, got: %s", hash1)
	}

	// Test that same password generates different hashes
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Second HashPassword failed: %v", err)
	}
	if hash1 == hash2 {
		t.Error("Same password should generate different hashes due to random salt")
	}
}

func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		hash      string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "Valid password",
			password:  "correctPassword123!",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "Wrong password",
			password:  "wrongPassword123!",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "Empty password",
			password:  "",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "Malformed hash",
			password:  "test",
			hash:      "invalid-hash",
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// If no hash provided, generate one
			hash := tt.hash
			var err error
			if hash == "" {
				hash, err = HashPassword("correctPassword123!")
				if err != nil {
					t.Fatalf("Failed to generate hash: %v", err)
				}
			}

			valid, err := VerifyPassword(tt.password, hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if valid != tt.wantValid {
				t.Errorf("VerifyPassword() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestDecodeHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		want    *params
		wantErr bool
	}{
		{
			name: "Valid hash",
			hash: "$argon2id$v=19$m=1048576,t=4,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			want: &params{
				memory:      1048576,
				iterations:  4,
				parallelism: 4,
				saltLength:  16,
				keyLength:   32,
			},
			wantErr: false,
		},
		{
			name:    "Invalid format",
			hash:    "invalid-hash",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid base64",
			hash:    "$argon2id$v=19$m=1048576,t=4,p=4$invalid!base64$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Different parameters",
			hash: "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			want: &params{
				memory:      65536,
				iterations:  3,
				parallelism: 2,
				saltLength:  16,
				keyLength:   32,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, salt, hash, err := decodeHash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Validate all parameters
			if params.memory != tt.want.memory {
				t.Errorf("memory = %v, want %v", params.memory, tt.want.memory)
			}
			if params.iterations != tt.want.iterations {
				t.Errorf("iterations = %v, want %v", params.iterations, tt.want.iterations)
			}
			if params.parallelism != tt.want.parallelism {
				t.Errorf("parallelism = %v, want %v", params.parallelism, tt.want.parallelism)
			}
			if len(salt) != int(tt.want.saltLength) {
				t.Errorf("salt length = %v, want %v", len(salt), tt.want.saltLength)
			}
			if len(hash) != int(tt.want.keyLength) {
				t.Errorf("hash length = %v, want %v", len(hash), tt.want.keyLength)
			}
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "mySecurePassword123!"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hash, err := HashPassword(password)
		if err != nil {
			b.Fatalf("HashPassword failed: %v", err)
		}
		// Prevent compiler optimization
		if hash == "" {
			b.Fatal("empty hash")
		}
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "mySecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		b.Fatalf("HashPassword failed: %v", err)
	}

	benchmarks := []struct {
		name     string
		password string
		hash     string
	}{
		{
			name:     "Correct password",
			password: password,
			hash:     hash,
		},
		{
			name:     "Wrong password",
			password: "wrongPassword123!",
			hash:     hash,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				valid, err := VerifyPassword(bm.password, bm.hash)
				if err != nil {
					b.Fatalf("VerifyPassword failed: %v", err)
				}
				// Prevent compiler optimization
				if valid && bm.name == "Wrong password" {
					b.Fatal("wrong password validated successfully")
				}
			}
		})
	}
}

func BenchmarkParallelVerifyPassword(b *testing.B) {
	password := "mySecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		b.Fatalf("HashPassword failed: %v", err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			valid, err := VerifyPassword(password, hash)
			if err != nil {
				b.Fatalf("VerifyPassword failed: %v", err)
			}
			if !valid {
				b.Fatal("valid password not verified")
			}
		}
	})
}
