#!/usr/bin/env python3
"""
Tracing Demo - Frontend Service
Receives user requests and forwards to middleware service.
Demonstrates OpenTelemetry auto-instrumentation with Flask.
"""
from flask import Flask, jsonify, request
import requests
import os
import logging

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
    "service.name": "frontend",
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

# Get tracer for manual instrumentation if needed
tracer = trace.get_tracer(__name__)

# Create Flask application
app = Flask(__name__)

# Auto-instrument Flask to create spans for HTTP requests
# Exclude health check endpoint to reduce tracing noise
FlaskInstrumentor().instrument_app(app, excluded_urls="/health")

# Auto-instrument requests library to propagate trace context
RequestsInstrumentor().instrument()

# Configuration for downstream services
MIDDLEWARE_URL = os.getenv('MIDDLEWARE_URL',
                          'http://middleware-service.services.svc.cluster.local:5001')

@app.route('/')
def index():
    """
    Health check and service information endpoint.
    Returns service metadata.
    """
    return jsonify({
        "service": "frontend",
        "version": "1.0.0",
        "message": "Tracing demo frontend service",
        "trace_enabled": True,
        "middleware_url": MIDDLEWARE_URL
    })

@app.route('/health')
def health():
    """
    Kubernetes health check endpoint.
    Used by readiness and liveness probes.
    """
    return jsonify({"status": "healthy"}), 200

@app.route('/api/process', methods=['GET', 'POST'])
def process():
    """
    Main processing endpoint that demonstrates distributed tracing.
    Forwards request to middleware service and returns combined response.
    Creates a custom span to track the processing logic.
    """
    # Create a custom span for the processing logic
    with tracer.start_as_current_span("frontend-process") as span:
        # Add custom attributes to the span for better observability
        span.set_attribute("frontend.handler", "process")

        # Get request data (supports both GET and POST)
        if request.method == 'POST':
            data = request.get_json() or {}
        else:
            data = {"source": "GET request"}

        span.set_attribute("frontend.request.method", request.method)

        try:
            # Call middleware service
            # The requests library auto-instrumentation will create a span
            # and propagate the trace context via W3C Trace Context headers
            logger.info(f"Calling middleware at {MIDDLEWARE_URL}/api/transform")

            response = requests.post(
                f'{MIDDLEWARE_URL}/api/transform',
                json=data,
                timeout=10
            )

            response.raise_for_status()
            middleware_data = response.json()

            # Record successful call in span
            span.set_attribute("frontend.middleware.status", response.status_code)

            return jsonify({
                "service": "frontend",
                "status": "success",
                "request_data": data,
                "middleware_response": middleware_data
            }), 200

        except requests.exceptions.RequestException as e:
            # Log error and record in span
            logger.error(f"Error calling middleware: {e}")
            span.set_attribute("frontend.error", str(e))

            # Set span status to error
            span.set_status(trace.Status(trace.StatusCode.ERROR, str(e)))

            return jsonify({
                "service": "frontend",
                "status": "error",
                "error": str(e)
            }), 500

if __name__ == '__main__':
    logger.info("Starting frontend service on port 5000")
    logger.info(f"Middleware URL: {MIDDLEWARE_URL}")
    logger.info(f"OTLP endpoint: {os.getenv('OTEL_EXPORTER_OTLP_ENDPOINT', 'default')}")
    app.run(host='0.0.0.0', port=5000, debug=False)
