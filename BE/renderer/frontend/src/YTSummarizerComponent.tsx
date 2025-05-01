// eslint-disable-next-line @typescript-eslint/no-unused-vars
import { useState } from "react"
import "./App.css"

function YTSummarizerComponent() {
  return (
    <form
      method="POST"
      action="/summarize"
      className="flex items-center justify-center gap-2 w-full"
    >
      <input
        type="text"
        name="videoUrl"
        placeholder="Enter YouTube URL (e.g., https://www.youtube.com/watch?v=dQw4w9WgXcQ)"
        className="w-3/4 p-3 border rounded-lg shadow-sm focus:ring-2 focus:ring-red-500 focus:border-transparent"
        required
      />
      <button
        type="submit"
        className="bg-red-600 hover:bg-red-700 text-white px-6 py-3 rounded-lg shadow-md transition-colors duration-200 flex items-center justify-center min-w-32"
      >
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
      </button>
    </form>
  )
}

export default YTSummarizerComponent
