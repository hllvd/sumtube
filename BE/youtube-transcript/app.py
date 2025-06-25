from flask import Flask, request, jsonify, Response
from youtube_transcript_api import YouTubeTranscriptApi, TranscriptsDisabled, NoTranscriptFound
from youtube_transcript_api.proxies import GenericProxyConfig
import datetime
import os

app = Flask(__name__)

# Proxy configuration via env var
proxy_url = os.getenv('PROXY_SERVER')
print(f"[DEBUG] PROXY_SERVER={proxy_url}")

# Configure YouTubeTranscriptApi instance
if proxy_url:
    print(f"[Proxy Enabled] Using proxy: {proxy_url}")
    ytt_api = YouTubeTranscriptApi(
        proxy_config=GenericProxyConfig(
            http_url=proxy_url,
            https_url=proxy_url,
        )
    )
else:
    print("[Proxy Disabled] No proxy configured")
    ytt_api = YouTubeTranscriptApi()

def convert_to_srt(transcript):
    srt = ""
    for i, entry in enumerate(transcript):
        start = str(datetime.timedelta(seconds=int(entry['start'])))
        end = str(datetime.timedelta(seconds=int(entry['start'] + entry['duration'])))
        text = entry['text']
        srt += f"{i+1}\n{start},000 --> {end},000\n{text}\n\n"
    return srt

@app.route('/transcript', methods=['POST'])
def transcript():
    video_id = request.args.get('vid')
    lang = request.args.get('lang')
    fmt = request.args.get('format', 'json')

    if not video_id or not lang:
        return jsonify({'error': 'Missing vid or lang parameter'}), 400

    try:
        transcript = ytt_api.get_transcript(video_id, languages=[lang])
    except TranscriptsDisabled:
        return jsonify({'error': 'Transcripts are disabled for this video'}), 403
    except NoTranscriptFound:
        return jsonify({'error': 'Transcript not found for the given language'}), 404
    except Exception as e:
        return jsonify({'error': str(e)}), 500

    if fmt == 'srt':
        srt_text = convert_to_srt(transcript)
        return Response(srt_text, mimetype='text/plain')
    else:
        return jsonify(transcript)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5050, debug=True)
