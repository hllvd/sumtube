from flask import Flask, request, jsonify, Response
from youtube_transcript_api import YouTubeTranscriptApi, TranscriptsDisabled, NoTranscriptFound
import datetime
import os
import requests

app = Flask(__name__)

# Proxy configuration
proxy_server = os.getenv('PROXY_SERVER')
proxies = {
    'http': proxy_server,
    'https': proxy_server
} if proxy_server else None

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
        # Configure YouTubeTranscriptApi to use the proxy
        if proxy_server:
            YouTubeTranscriptApi.http_client.proxies = proxies
            
        transcript = YouTubeTranscriptApi.get_transcript(video_id, languages=[lang])
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
    app.run(host='0.0.0.0', port=5050, debug=False)
