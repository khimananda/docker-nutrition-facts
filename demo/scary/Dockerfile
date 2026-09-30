# A deliberately unhealthy image for the demo. Do not copy this.
FROM ubuntu:latest
ENV API_TOKEN=changeme DB_PASSWORD=changeme
RUN apt-get update && apt-get install -y curl sudo openssh-server
RUN curl -fsSL https://example.com/install.sh | sh || true
RUN pip install flask || true
CMD ["bash"]
