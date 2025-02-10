export type Role = 'gm' | 'player'

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  success: boolean;
  error?: string;
}

export interface AuthResponse {
  authenticated: boolean;
  role?: Role;
}
