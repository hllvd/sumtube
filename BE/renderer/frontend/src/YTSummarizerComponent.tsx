import { useState } from "react"
import "./App.css"

function YTSummarizerComponent() {
  const [videoUrl, setVideoUrl] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [videoInfo, setVideoInfo] = useState<null | {
    videoId: string
    uploader_id: string
    title: string
    duration: string
  }>(null)

  const extractVideoId = (url: string) => {
    const regExp =
      /^.*(youtu.be\/|v\/|u\/\w\/|embed\/|watch\?v=|&v=)([^#&?]*).*/
    const match = url.match(regExp)
    return match && match[2].length === 11 ? match[2] : null
  }

  const fetchSummary = async (videoId: string, language: string) => {
    try {
      const response = await fetch("http://localhost:8080/summary", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ videoId, language }),
      })

      const data = await response.json()

      if (data.status === "processing") {
        setVideoInfo({
          videoId: data.videoId,
          uploader_id: data.uploader_id,
          title: data.title,
          duration: data.duration,
        })
        setTimeout(() => fetchSummary(videoId, language), 3000)
      } else if (data.status === "completed") {
        window.location.href = `${window.location.origin}/${data.lang}/${data.path}/${data.videoId}`
      }
    } catch (error) {
      console.error("Error fetching summary:", error)
      setIsLoading(false)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const videoId = extractVideoId(videoUrl)
    if (!videoId) return alert("Invalid YouTube URL")

    setIsLoading(true)
    setVideoInfo(null) // reset
    const root = document.getElementById("react-root")
    const language = root?.dataset.lang || "en"
    fetchSummary(videoId, language)
  }

  return (
    <>
      <form
        onSubmit={handleSubmit}
        className="flex items-center justify-center gap-2 w-full flex-wrap"
      >
        <input
          type="text"
          value={videoUrl}
          onChange={(e) => setVideoUrl(e.target.value)}
          placeholder="Enter YouTube URL (e.g., https://www.youtube.com/watch?v=ANG-lEmc0eQ)"
          className="w-full md:w-3/4 p-3 border rounded-lg shadow-sm focus:ring-2 focus:ring-red-500 focus:border-transparent"
          required
        />
        <button
          type="submit"
          disabled={isLoading}
          className="bg-red-600 hover:bg-red-700 text-white px-6 py-3 rounded-lg shadow-md transition-colors duration-200 flex items-center justify-center min-w-32 disabled:opacity-50"
        >
          {isLoading ? (
            <div className="flex items-center gap-2">
              <div className="spinner" />
              <span>Processing...</span>
            </div>
          ) : (
            <span className="flex items-center">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-5 w-5 mr-1"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z"
                  clipRule="evenodd"
                />
              </svg>
              Summarize
            </span>
          )}
        </button>
      </form>

      {/* Extra spacing */}
      <div className="mt-4" />

      {isLoading && (
        <div className="w-full text-center max-w-xl mx-auto bg-white p-6 rounded-lg shadow-md">
          {/* Persistent spinner */}
          <div className="spinner mb-4" />

          {videoInfo ? (
            <>
              <img
                src={`https://i.ytimg.com/vi/${videoInfo.videoId}/hqdefault.jpg`}
                alt="Video thumbnail"
                className="rounded-md mb-4 w-full"
              />
              <h2 className="text-lg font-semibold mb-2">{videoInfo.title}</h2>
              <p className="text-gray-600 mb-1">
                <strong>Uploader:</strong> {videoInfo.uploader_id}
              </p>
              <p className="text-gray-600 mb-2">
                <strong>Duration:</strong> {videoInfo.duration} sec
              </p>
              <p className="text-red-500 font-medium">
                Summarizing video… please wait
              </p>
            </>
          ) : (
            <p className="text-gray-600">Loading video info…</p>
          )}
        </div>
      )}
    </>
  )
}

export default YTSummarizerComponent
