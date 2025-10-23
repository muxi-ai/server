# Dummy MUXI Runtime SIF - Minimal Test Image
# This is a simplified test image that packages dummy_app.py
# The full runtime SIF will be built in the runtime repo

FROM python:3.10-slim

# Set working directory
WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy dummy application
COPY dummy_app.py .

# Environment variables (will be overridden by MUXI Server)
ENV PORT=8000
ENV HOST=0.0.0.0
ENV FORMATION_ID=dummy

# Default command (overridden by MUXI Server with actual formation code)
CMD ["python", "dummy_app.py"]
