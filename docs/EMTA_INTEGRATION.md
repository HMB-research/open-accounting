# e-MTA Automatic TSD Submission

> **Status: BLOCKED / NEEDS TESTING**
>
> This feature requires Estonian X-Road access credentials and a test environment to implement and verify.

## Overview

The Estonian Tax and Customs Board (Maksu- ja Tolliamet, MTA) provides electronic services through the e-MTA portal. Automatic TSD (Tulu- ja sotsiaalmaksu deklaratsioon) submission requires integration with Estonia's X-Road (X-tee) data exchange layer.

## Current Implementation

### Completed

- [x] TSD data model and database schema (`migrations/007_payroll.up.sql`)
- [x] TSD calculation from payroll runs (`internal/payroll/tsd.go`)
- [x] TSD XML export in e-MTA format (`internal/payroll/tsd_export.go`)
- [x] TSD CSV export alternative
- [x] Estonian personal code (isikukood) validation
- [x] API endpoints for TSD management:
  - `GET /api/v1/tenants/{tenantID}/tsd` - List declarations
  - `GET /api/v1/tenants/{tenantID}/tsd/{year}/{month}` - Get specific declaration
  - `GET /api/v1/tenants/{tenantID}/tsd/{year}/{month}/xml` - Export XML
  - `GET /api/v1/tenants/{tenantID}/tsd/{year}/{month}/csv` - Export CSV
  - `POST /api/v1/tenants/{tenantID}/tsd/{year}/{month}/submit` - Mark as submitted

### Manual Workflow (Currently Supported)

1. Generate payroll run for the period
2. Calculate payroll taxes
3. Generate TSD declaration
4. Export TSD as XML via API
5. Manually upload XML to e-MTA portal
6. Mark declaration as submitted in the system

## X-Road Integration Requirements

### Authentication

X-Road services require one of the following authentication methods:

| Method | Description | Use Case |
|--------|-------------|----------|
| ID-card | Estonian digital ID card with PIN | Personal authentication |
| Mobile-ID | Mobile phone-based authentication | Personal authentication |
| Smart-ID | App-based authentication | Personal authentication |
| Organizational Certificate | X.509 certificate issued to the organization | Machine-to-machine (required for automatic submission) |

**For automatic submission, an organizational X-Road certificate is required.**

### Required X-Road Services

The following X-Road services are needed for automatic TSD submission:

#### 1. `uploadMime` - Upload Declaration

```
Service: emta-v6/uploadMime
Purpose: Upload TSD XML document to e-MTA
Input: MIME-encoded XML document
Output: Document reference ID
```

#### 2. `confirmTsd` - Confirm Submission

```
Service: emta-v6/confirmTsd
Purpose: Confirm and submit the uploaded declaration
Input: Document reference ID, confirmation flag
Output: Submission confirmation, reference number
```

#### 3. `getTsdStatus` - Check Status

```
Service: emta-v6/getTsdStatus
Purpose: Query the processing status of a submitted declaration
Input: Document reference ID or submission reference
Output: Status (PENDING, PROCESSING, ACCEPTED, REJECTED)
```

#### 4. `getTsdFeedback` - Get Validation Feedback

```
Service: emta-v6/getTsdFeedback
Purpose: Retrieve validation errors or acceptance confirmation
Input: Document reference ID
Output: List of errors/warnings, acceptance details
```

### X-Road Message Format

X-Road uses SOAP-based messaging with specific headers:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
  <SOAP-ENV:Header>
    <xrd:client xrd:objectType="SUBSYSTEM">
      <xrd:xRoadInstance>EE</xrd:xRoadInstance>
      <xrd:memberClass>COM</xrd:memberClass>
      <xrd:memberCode>REGISTRY_CODE</xrd:memberCode>
      <xrd:subsystemCode>SUBSYSTEM_CODE</xrd:subsystemCode>
    </xrd:client>
    <xrd:service xrd:objectType="SERVICE">
      <xrd:xRoadInstance>EE</xrd:xRoadInstance>
      <xrd:memberClass>GOV</xrd:memberClass>
      <xrd:memberCode>70000349</xrd:memberCode>
      <xrd:subsystemCode>emta</xrd:subsystemCode>
      <xrd:serviceCode>uploadMime</xrd:serviceCode>
      <xrd:serviceVersion>v6</xrd:serviceVersion>
    </xrd:service>
    <xrd:id>unique-message-id</xrd:id>
    <xrd:protocolVersion>4.0</xrd:protocolVersion>
  </SOAP-ENV:Header>
  <SOAP-ENV:Body>
    <!-- Service-specific content -->
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>
```

## Proposed Implementation

### New Package Structure

```
internal/
└── emta/
    ├── client.go       # X-Road SOAP client
    ├── types.go        # Request/response types
    ├── upload.go       # uploadMime implementation
    ├── confirm.go      # confirmTsd implementation
    ├── status.go       # getTsdStatus implementation
    ├── feedback.go     # getTsdFeedback implementation
    └── config.go       # Configuration and credentials
```

### Configuration

```go
type EMTAConfig struct {
    // X-Road Security Server URL
    SecurityServerURL string `json:"security_server_url"`

    // Client identification
    XRoadInstance  string `json:"xroad_instance"`   // EE
    MemberClass    string `json:"member_class"`     // COM or NGO
    MemberCode     string `json:"member_code"`      // Registry code
    SubsystemCode  string `json:"subsystem_code"`   // Registered subsystem

    // Certificate paths
    CertificatePath string `json:"certificate_path"`
    PrivateKeyPath  string `json:"private_key_path"`

    // Optional: CA certificate for verification
    CACertificatePath string `json:"ca_certificate_path,omitempty"`
}
```

### API Endpoints (Proposed)

```
POST /api/v1/tenants/{tenantID}/tsd/{year}/{month}/auto-submit
  - Uploads XML to e-MTA
  - Confirms submission
  - Returns submission reference

GET /api/v1/tenants/{tenantID}/tsd/{year}/{month}/emta-status
  - Queries current status from e-MTA
  - Updates local status accordingly

POST /api/v1/tenants/{tenantID}/settings/emta
  - Configure e-MTA credentials for tenant

GET /api/v1/tenants/{tenantID}/settings/emta/test
  - Test X-Road connectivity
```

### Database Changes

```sql
-- Add e-MTA tracking to TSD declarations
ALTER TABLE tsd_declarations ADD COLUMN emta_document_id VARCHAR(100);
ALTER TABLE tsd_declarations ADD COLUMN emta_submission_id VARCHAR(100);
ALTER TABLE tsd_declarations ADD COLUMN emta_last_checked_at TIMESTAMPTZ;
ALTER TABLE tsd_declarations ADD COLUMN emta_errors JSONB;

-- Add e-MTA configuration to tenant settings
-- (stored in tenant_settings JSONB field)
```

## Blockers

### 1. X-Road Access

- [ ] Register organization with X-Road
- [ ] Obtain organizational certificate
- [ ] Register subsystem for e-MTA access
- [ ] Configure security server access

### 2. Test Environment

- [ ] Access to e-MTA test environment (https://test-emta.ee)
- [ ] Test X-Road instance access
- [ ] Sample test data with valid personal codes

### 3. Documentation

- [ ] Obtain official e-MTA X-Road service WSDL files
- [ ] Verify current API version (v6 assumed)
- [ ] Confirm XML schema requirements for 2025

## Testing Requirements

### Unit Tests

- XML generation matches e-MTA schema
- SOAP envelope construction
- Error response handling
- Status mapping

### Integration Tests (Requires Test Environment)

- X-Road connectivity
- Certificate authentication
- Full submission workflow
- Error scenarios (validation failures, duplicate submissions)

### Manual Testing Checklist

- [ ] Generate TSD for test period
- [ ] Export XML and validate against schema
- [ ] Upload to e-MTA test environment
- [ ] Verify acceptance/rejection handling
- [ ] Check status polling
- [ ] Verify feedback retrieval

## Resources

### Official Documentation

- [e-MTA Portal](https://www.emta.ee/en)
- [X-Road Documentation](https://x-tee.ee/docs/)
- [Estonian Tax Forms](https://www.emta.ee/en/business-client/taxes-and-payment/tax-returns)

### Technical References

- X-Road Protocol: Version 4.0
- TSD XML Schema: Based on 01.01.2025 specification
- SOAP Version: 1.1

## Timeline

This feature is **blocked** pending:

1. **Organizational X-Road registration** - Required for machine-to-machine communication
2. **Test environment access** - Cannot develop without ability to test
3. **Official WSDL/schema files** - Need current service definitions

Once blockers are resolved, estimated implementation time: **3-5 days**

## Contact

For X-Road registration and e-MTA API access:
- Estonian Information System Authority (RIA): https://www.ria.ee
- e-MTA Support: emta@emta.ee
