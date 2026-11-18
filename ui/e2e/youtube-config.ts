export interface YouTubeTestConfig {
  link: string
}

export function youtubeTestConfig(environment: NodeJS.ProcessEnv = process.env): YouTubeTestConfig {
  const link = environment.INTEGRATION_TEST_YT_DLP_LINK?.trim()
  if (!link) throw new Error('Missing INTEGRATION_TEST_YT_DLP_LINK')
  let url: URL
  try {
    url = new URL(link)
  } catch {
    throw new Error('INTEGRATION_TEST_YT_DLP_LINK must contain a YouTube video URL')
  }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.port ||
    !['youtube.com', 'www.youtube.com', 'm.youtube.com', 'music.youtube.com', 'youtu.be'].includes(url.hostname)) {
    throw new Error('INTEGRATION_TEST_YT_DLP_LINK must contain a YouTube video URL')
  }
  return { link }
}
