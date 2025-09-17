# 📌 MML Model API

This project provides a simple internal API that allows you to send a request with a prompt template and input data.  
It loads **system** and **user** prompt templates, sends them to the model, and returns the response along with metadata.

---

## 🚀 How to Run

### 1. Clone or copy this project

Since this project is local-only, make sure you have the source files in a directory called `mml-model`.

### 2. Create prompt templates

Create your prompt templates in `llm-model/promps`.

Example expected files:

- `prompt1.user.prompt.txt`
- `prompt1.system.prompt.txt`

Each template file contains plain text that will be inserted into the request.

### 3. Run with Docker Compose

Start the service:

```bash
./start.sh up
```

### Example curl

```
 curl -X POST http://localhost:8080/generate \
  -H "Content-Type: application/json" \
  -d '{
    "template": "prompt1",
    "input": {
      "language": "pt",
      "title": "this is a title",
      "captions": "lkjsdlfkjsdlkfjsdkfjsdklf"
    }
  }'

```

### Response example

```
{
  "result": "Generated text output here...",
  "request_duration": "123ms",
  "input": {
    "language": "pt",
    "title": "this is a title",
    "captions": "lkjsdlfkjsdlkfjsdkfjsdklf"
  }
}

```
