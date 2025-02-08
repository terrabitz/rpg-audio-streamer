export interface User {
  login: string;
  name: string;
  avatar_url: string;
}

export interface AuthResponse {
  authenticated: boolean;
  authorized: boolean;
  user: User | null;
}
