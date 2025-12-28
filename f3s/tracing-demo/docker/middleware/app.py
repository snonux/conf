#!/usr/bin/env python3
"""
Tracing Demo - Middleware Service
Transforms data and calls backend service.
Demonstrates trace context propagation in a multi-tier architecture.
"""
from flask import Flask, jsonify, request
import requests
import os
import logging
import time

# OpenTelemetry imports for distributed tracing
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk.resources import Resource

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize OpenTelemetry tracing with resource attributes
# These attributes identify this service in traces
resource = Resource(attributes={
    "service.name": "middleware",
    "service.namespace": "tracing-demo",
    "service.version": "1.0.0",
    "deployment.environment": "production"
})

provider = TracerProvider(resource=resource)

# Configure OTLP exporter to send traces to Alloy
otlp_exporter = OTLPSpanExporter(
    endpoint=os.getenv('OTEL_EXPORTER_OTLP_ENDPOINT',
                      'http://alloy.monitoring.svc.cluster.local:4317'),
    insecure=True
)

# Batch spans for efficient export
processor = BatchSpanProcessor(otlp_exporter)
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

# Get tracer for manual instrumentation
tracer = trace.get_tracer(__name__)

# Create Flask application
app = Flask(__name__)

# Auto-instrument Flask and requests library
# Auto-instrument Flask to create spans for HTTP requests
# Exclude health check endpoint to reduce tracing noise
FlaskInstrumentor().instrument_app(app, excluded_urls="/health")
RequestsInstrumentor().instrument()

# Configuration for downstream services
BACKEND_URL = os.getenv('BACKEND_URL',
                       'http://backend-service.services.svc.cluster.local:5002')

@app.route('/')
def index():
    """
    Health check and service information endpoint.
    Returns service metadata.
    """
    return jsonify({
        "service": "middleware",
        "version": "1.0.0",
        "message": "Tracing demo middleware service",
        "backend_url": BACKEND_URL
    })

@app.route('/health')
def health():
    """
    Kubernetes health check endpoint.
    Used by readiness and liveness probes.
    """
    return jsonify({"status": "healthy"}), 200

@app.route('/api/transform', methods=['POST'])
def transform():
    """
    Transform data and fetch additional data from backend.
    Demonstrates trace context propagation through multiple services.
    Creates custom spans to track transformation logic.
    """
    # Create a custom span for the transformation logic
    with tracer.start_as_current_span("middleware-transform") as span:
        # Add custom attributes to the span
        span.set_attribute("middleware.handler", "transform")

        # Get request data from frontend
        data = request.get_json() or {}
        span.set_attribute("middleware.input.keys", str(list(data.keys())))

        # Simulate some data transformation processing
        time.sleep(0.05)

        try:
            # Call backend service to fetch additional data
            # The trace context is automatically propagated via HTTP headers
            logger.info(f"Calling backend at {BACKEND_URL}/api/data")

            response = requests.get(
                f'{BACKEND_URL}/api/data',
                timeout=10
            )

            response.raise_for_status()
            backend_data = response.json()

            # Record successful call in span
            span.set_attribute("middleware.backend.status", response.status_code)

            # Transform and combine the data
            transformed = {
                "middleware_processed": True,
                "original_data": data,
                "backend_data": backend_data,
                "transformation_time_ms": 50
            }

            return jsonify(transformed), 200

        except requests.exceptions.RequestException as e:
            # Log error and record in span
            logger.error(f"Error calling backend: {e}")
            span.set_attribute("middleware.error", str(e))

            # Set span status to error
            span.set_status(trace.Status(trace.StatusCode.ERROR, str(e)))

            return jsonify({
                "service": "middleware",
                "status": "error",
                "error": str(e)
            }), 500

if __name__ == '__main__':
    logger.info("Starting middleware service on port 5001")
    logger.info(f"Backend URL: {BACKEND_URL}")
    logger.info(f"OTLP endpoint: {os.getenv('OTEL_EXPORTER_OTLP_ENDPOINT', 'default')}")
    app.run(host='0.0.0.0', port=5001, debug=False)
