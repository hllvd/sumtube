export const extractVideoId = (url: string) => {
  try {
    // Normalize the URL
    const cleanedUrl = url.trim()

    // Match common YouTube patterns including shorts
    const regExp =
      /(?:youtu\.be\/|youtube\.com\/(?:watch\?(?:.*&)?v=|v\/|embed\/|live\/|shorts\/))([a-zA-Z0-9_-]{11})/

    const match = cleanedUrl.match(regExp)
    if (match && match[1]) {
      return match[1]
    }

    // Fallback for full watch URLs
    const urlObj = new URL(cleanedUrl)
    return urlObj.searchParams.get("v")
  } catch {
    return null
  }
}
