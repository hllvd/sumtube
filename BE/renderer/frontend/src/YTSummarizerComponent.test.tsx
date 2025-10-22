import { extractVideoId } from "./utils/youtube"
import { describe, expect, test } from "@jest/globals"

describe("extractVideoId", () => {
  test("extracts video ID from standard YouTube watch URL", () => {
    const url = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("extracts video ID from YouTube short URL", () => {
    const url = "https://youtu.be/dQw4w9WgXcQ"
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("extracts video ID from YouTube shorts URL", () => {
    const url = "https://www.youtube.com/shorts/_KihTNR5R9g"
    expect(extractVideoId(url)).toBe("_KihTNR5R9g")
  })

  test("extracts video ID from YouTube embed URL", () => {
    const url = "https://www.youtube.com/embed/dQw4w9WgXcQ"
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("extracts video ID from YouTube live URL", () => {
    const url = "https://www.youtube.com/live/dQw4w9WgXcQ"
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("extracts video ID from URL with additional query parameters", () => {
    const url =
      "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=123&feature=shared"
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("handles URLs with extra whitespace", () => {
    const url = "  https://www.youtube.com/watch?v=dQw4w9WgXcQ  "
    expect(extractVideoId(url)).toBe("dQw4w9WgXcQ")
  })

  test("returns null for invalid YouTube URLs", () => {
    const invalidUrls = [
      "https://example.com",
      "not a url",
      "https://youtube.com",
      "https://www.youtube.com/channel/123",
      "",
      " ",
    ]

    invalidUrls.forEach((url) => {
      expect(extractVideoId(url)).toBeNull()
    })
  })

  test("handles malformed URLs gracefully", () => {
    const malformedUrls = [
      "youtube.com/watch?v=",
      "https://youtube.com/watch",
      "https://youtube.com/watch?",
      "https://youtube.com/shorts/",
    ]

    malformedUrls.forEach((url) => {
      expect(extractVideoId(url)).toBeNull()
    })
  })
})
