FROM python:3.12-slim

WORKDIR /srv

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .
COPY run.sh .
RUN chmod +x run.sh

EXPOSE 8000

ENTRYPOINT ["./run.sh"]
