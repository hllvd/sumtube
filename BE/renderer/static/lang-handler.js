;(() => {
  const supportedLangs = [
    "pt",
    "en",
    "fr",
    "es",
    "it",
    "de",
    "ru",
    "ar",
    "ja",
    "zh",
    "ko",
  ]

  // Cookie helpers
  function getCookie(name) {
    const match = document.cookie.match(new RegExp("(^| )" + name + "=([^;]+)"))
    return match ? decodeURIComponent(match[2]) : null
  }
  function setCookie(name, value, days = 365) {
    const expires = new Date(Date.now() + days * 864e5).toUTCString()
    document.cookie = `${name}=${encodeURIComponent(
      value
    )}; expires=${expires}; path=/`
  }

  // Browser language detection (fallback to 'en')
  function detectBrowserLang() {
    const lang = (
      navigator.language ||
      navigator.userLanguage ||
      "en"
    ).toLowerCase()
    const map = {
      pt: "pt",
      "pt-br": "pt",
      en: "en",
      "en-us": "en",
      fr: "fr",
      es: "es",
      "es-es": "es",
      it: "it",
      de: "de",
      ru: "ru",
      ar: "ar",
      ja: "ja",
      zh: "zh",
      "zh-cn": "zh",
      ko: "ko",
    }
    return map[lang] || map[lang.split("-")[0]] || "en"
  }

  function handleRedirect() {
    const path = window.location.pathname || "/"
    const currentLang = supportedLangs.find(
      (l) => path === "/" + l || path.startsWith("/" + l + "/")
    )
    if (currentLang) {
      setCookie("language", currentLang)
      return
    }

    const rawHref = window.location.href
    let decodedHref
    try {
      decodedHref = decodeURIComponent(rawHref)
    } catch (e) {
      decodedHref = rawHref
    }

    const idRegexes = [
      /[?&]v=([A-Za-z0-9_-]{11})(?:[&#]|$)/i,
      /(?:youtu\.be\/)([A-Za-z0-9_-]{11})(?:[?&#]|$)/i,
      /(?:youtube\.com\/embed\/)([A-Za-z0-9_-]{11})(?:[?&#]|$)/i,
      /\/([A-Za-z0-9_-]{11})(?:[/?#]|$)/i,
    ]

    let videoId = null
    for (const rx of idRegexes) {
      const m = decodedHref.match(rx)
      if (m && m[1]) {
        videoId = m[1]
        break
      }
    }

    if (videoId) {
      const cookieLang = getCookie("language")
      const lang =
        cookieLang && supportedLangs.includes(cookieLang)
          ? cookieLang
          : detectBrowserLang()
      window.location.replace("/" + lang + "/" + videoId)
      return
    }

    if (path === "/" || path === "") {
      const cookieLang = getCookie("language")
      if (cookieLang && supportedLangs.includes(cookieLang)) {
        window.location.replace("/" + cookieLang)
        return
      }
      window.location.replace("/" + detectBrowserLang())
    }
  }

  // Ensure it runs after DOM and location are ready
  if (
    document.readyState === "complete" ||
    document.readyState === "interactive"
  ) {
    handleRedirect()
  } else {
    document.addEventListener("DOMContentLoaded", handleRedirect)
  }
})()
