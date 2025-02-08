export interface User {
  sub: number
  name: string
  login: string
  email: string
  avatar_url: string
  exp: number
}

export interface AuthResponse {
  authenticated: boolean
  user: User | null
}
