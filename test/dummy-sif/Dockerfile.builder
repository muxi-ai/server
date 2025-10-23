# SIF Builder Container
# Uses official Singularity image to build SIF files on macOS

# Use official Singularity image (amd64)
FROM --platform=linux/amd64 quay.io/singularity/singularity:v3.11.4

# Set working directory
WORKDIR /build

# Copy definition file and dependencies
COPY muxi-runtime-dummy.def /build/
COPY requirements.txt /build/
COPY dummy_app.py /build/

# Build the SIF file
# Override entrypoint to use shell
ENTRYPOINT ["/bin/sh", "-c"]

# This will be executed when the container runs
CMD ["singularity build /output/muxi-runtime-dummy.sif /build/muxi-runtime-dummy.def && ls -lh /output/"]
