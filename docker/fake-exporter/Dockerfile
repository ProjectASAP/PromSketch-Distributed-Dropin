FROM python:3.11-slim
RUN pip install --no-cache-dir prometheus-client numpy
WORKDIR /app
COPY fake_exporter.py .
EXPOSE 8080
ENTRYPOINT ["python", "fake_exporter.py"]
