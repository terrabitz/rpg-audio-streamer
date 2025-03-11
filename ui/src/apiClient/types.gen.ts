// This file is auto-generated by @hey-api/openapi-ts

export type LoginRequest = {
    username: string;
    password: string;
};

export type LoginResponse = {
    success: boolean;
    error: string;
};

export type JoinRequest = {
    token: string;
};

export type JoinTokenResponse = {
    token: string;
};

export type AuthStatusResponse = {
    authenticated: boolean;
    role: 'gm' | 'player';
};

export type Track = {
    id: string;
    createdAt: string;
    name: string;
    path: string;
    typeId: string;
};

export type TrackType = {
    id: string;
    name: string;
};

export type PostApiV1LoginData = {
    body: LoginRequest;
    path?: never;
    query?: never;
    url: '/api/v1/login';
};

export type PostApiV1LoginErrors = {
    /**
     * Invalid credentials
     */
    401: LoginResponse;
};

export type PostApiV1LoginError = PostApiV1LoginErrors[keyof PostApiV1LoginErrors];

export type PostApiV1LoginResponses = {
    /**
     * Login successful
     */
    200: LoginResponse;
};

export type PostApiV1LoginResponse = PostApiV1LoginResponses[keyof PostApiV1LoginResponses];

export type PostApiV1JoinData = {
    body: JoinRequest;
    path?: never;
    query?: never;
    url: '/api/v1/join';
};

export type PostApiV1JoinErrors = {
    /**
     * Invalid join token
     */
    401: unknown;
};

export type PostApiV1JoinResponses = {
    /**
     * Join successful
     */
    200: LoginResponse;
};

export type PostApiV1JoinResponse = PostApiV1JoinResponses[keyof PostApiV1JoinResponses];

export type GetApiV1AuthStatusData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/auth/status';
};

export type GetApiV1AuthStatusResponses = {
    /**
     * Authentication status
     */
    200: AuthStatusResponse;
};

export type GetApiV1AuthStatusResponse = GetApiV1AuthStatusResponses[keyof GetApiV1AuthStatusResponses];

export type PostApiV1AuthLogoutData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/auth/logout';
};

export type PostApiV1AuthLogoutResponses = {
    /**
     * Logout successful
     */
    200: unknown;
};

export type GetApiV1FilesData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/files';
};

export type GetApiV1FilesErrors = {
    /**
     * Not authorized
     */
    403: unknown;
};

export type GetApiV1FilesResponses = {
    /**
     * List of tracks
     */
    200: Array<Track>;
};

export type GetApiV1FilesResponse = GetApiV1FilesResponses[keyof GetApiV1FilesResponses];

export type PostApiV1FilesData = {
    body: {
        files?: Blob | File;
        name?: string;
        typeId?: string;
    };
    path?: never;
    query?: never;
    url: '/api/v1/files';
};

export type PostApiV1FilesErrors = {
    /**
     * Not authorized
     */
    403: unknown;
};

export type PostApiV1FilesResponses = {
    /**
     * File uploaded successfully
     */
    200: unknown;
};

export type DeleteApiV1FilesByTrackIdData = {
    body?: never;
    path: {
        trackID: string;
    };
    query?: never;
    url: '/api/v1/files/{trackID}';
};

export type DeleteApiV1FilesByTrackIdErrors = {
    /**
     * Not authorized
     */
    403: unknown;
    /**
     * Track not found
     */
    404: unknown;
};

export type DeleteApiV1FilesByTrackIdResponses = {
    /**
     * Track deleted successfully
     */
    200: unknown;
};

export type GetApiV1JoinTokenData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/join-token';
};

export type GetApiV1JoinTokenErrors = {
    /**
     * Not authorized
     */
    403: unknown;
};

export type GetApiV1JoinTokenResponses = {
    /**
     * Join token
     */
    200: JoinTokenResponse;
};

export type GetApiV1JoinTokenResponse = GetApiV1JoinTokenResponses[keyof GetApiV1JoinTokenResponses];

export type GetApiV1StreamByPathData = {
    body?: never;
    path: {
        path: string;
    };
    query?: never;
    url: '/api/v1/stream/{path}';
};

export type GetApiV1StreamByPathErrors = {
    /**
     * Not authorized
     */
    403: unknown;
    /**
     * File not found
     */
    404: unknown;
};

export type GetApiV1StreamByPathResponses = {
    /**
     * Audio stream
     */
    200: string;
};

export type GetApiV1StreamByPathResponse = GetApiV1StreamByPathResponses[keyof GetApiV1StreamByPathResponses];

export type GetApiV1TrackTypesData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/trackTypes';
};

export type GetApiV1TrackTypesErrors = {
    /**
     * Not authorized
     */
    403: unknown;
};

export type GetApiV1TrackTypesResponses = {
    /**
     * List of track types
     */
    200: Array<TrackType>;
};

export type GetApiV1TrackTypesResponse = GetApiV1TrackTypesResponses[keyof GetApiV1TrackTypesResponses];

export type GetApiV1WsData = {
    body?: never;
    path?: never;
    query?: never;
    url: '/api/v1/ws';
};

export type GetApiV1WsErrors = {
    /**
     * Not authorized
     */
    403: unknown;
};

export type ClientOptions = {
    baseUrl: 'https://skald.hackandsla.sh' | (string & {});
};