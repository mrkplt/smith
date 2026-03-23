FROM node:22-slim
RUN npm install -g @anthropic-ai/claude-code
RUN useradd -m -s /bin/bash claude
USER claude
WORKDIR /home/claude
