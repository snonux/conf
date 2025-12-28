#!/usr/bin/env python3
"""
Tracing Demo - Backend Service
Final service in the chain that returns data.
Simulates database queries and demonstrates end-to-end tracing.
"""
from flask import Flask, jsonify
import os
import logging
import time
from datetime import datetime

# OpenTelemetry imports for distributed tracing
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.sdk.resources import Resource

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize OpenTelemetry tracing with resource attributes
# These attributes identify this service in traces
resource = Resource(attributes={
    "service.name": "backend",
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

# Auto-instrument Flask
FlaskInstrumentor().instrument_app(app)

@app.route('/')
def index():
    """
    Health check and service information endpoint.
    Returns service metadata.
    """
    return jsonify({
        "service": "backend",
        "version": "1.0.0",
        "message": "Tracing demo backend service"
    })

@app.route('/health')
def health():
    """
    Kubernetes health check endpoint.
    Used by readiness and liveness probes.
    """
    return jsonify({"status": "healthy"}), 200

@app.route('/api/data', methods=['GET'])
def get_data():
    """
    Return data endpoint that simulates a database query.
    Creates custom spans to track query execution.
    This is the final service in the trace chain.
    """
    # Create a custom span for the database query simulation
    with tracer.start_as_current_span("backend-get-data") as span:
        # Add custom attributes to the span
        span.set_attribute("backend.handler", "get_data")

        # Simulate database query delay
        query_time = 0.1
        time.sleep(query_time)

        # Record query duration in span
        span.set_attribute("backend.query.duration_ms", query_time * 1000)
        span.set_attribute("backend.query.type", "simulated_database_query")

        # Prepare response data
        data = {
            "service": "backend",
            "data": {
                "id": 12345,
                "value": "Sample data from backend service",
                "timestamp": datetime.utcnow().isoformat(),
                "query_time_ms": query_time * 1000
            }
        }

        logger.info(f"Returning data: {data['data']['id']}")

        return jsonify(data), 200

if __name__ == '__main__':
    logger.info("Starting backend service on port 5002")
    logger.info(f"OTLP endpoint: {os.getenv('OTEL_EXPORTER_OTLP_ENDPOINT', 'default')}")
    app.run(host='0.0.0.0', port=5002, debug=False)
