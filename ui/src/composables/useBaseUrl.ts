export function useBaseUrl() {
  const getBaseUrl = () => {
    return `${window.location.protocol}//${window.location.host}`
  }

  return {
    getBaseUrl
  }
}
