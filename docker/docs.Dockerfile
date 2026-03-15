FROM python:3.12-slim

RUN pip install --no-cache-dir zensical

WORKDIR /workspace
ENTRYPOINT ["zensical"]
