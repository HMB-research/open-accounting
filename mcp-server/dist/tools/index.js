// API configuration - set via environment variables
const API_BASE = process.env.OPEN_ACCOUNTING_API_URL || 'http://localhost:8080';
const API_TOKEN = process.env.OPEN_ACCOUNTING_API_TOKEN || '';
// Tool definitions
export const tools = [
    {
        name: 'list_invoices',
        description: 'List invoices for a tenant with optional filters',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID to list invoices for',
                },
                status: {
                    type: 'string',
                    enum: ['DRAFT', 'SENT', 'PARTIALLY_PAID', 'PAID', 'OVERDUE', 'VOIDED'],
                    description: 'Filter by invoice status',
                },
                type: {
                    type: 'string',
                    enum: ['SALES', 'PURCHASE', 'CREDIT_NOTE'],
                    description: 'Filter by invoice type',
                },
                limit: {
                    type: 'number',
                    description: 'Maximum number of invoices to return',
                    default: 20,
                },
            },
            required: ['tenant_id'],
        },
    },
    {
        name: 'create_invoice',
        description: 'Create a new invoice',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID to create invoice for',
                },
                contact_id: {
                    type: 'string',
                    description: 'The contact (customer/vendor) ID',
                },
                invoice_type: {
                    type: 'string',
                    enum: ['SALES', 'PURCHASE', 'CREDIT_NOTE'],
                    description: 'Type of invoice',
                },
                issue_date: {
                    type: 'string',
                    description: 'Issue date (YYYY-MM-DD)',
                },
                due_date: {
                    type: 'string',
                    description: 'Due date (YYYY-MM-DD)',
                },
                lines: {
                    type: 'array',
                    items: {
                        type: 'object',
                        properties: {
                            description: { type: 'string' },
                            quantity: { type: 'string' },
                            unit_price: { type: 'string' },
                            vat_rate: { type: 'string' },
                        },
                        required: ['description', 'quantity', 'unit_price'],
                    },
                    description: 'Invoice line items',
                },
            },
            required: ['tenant_id', 'contact_id', 'invoice_type', 'issue_date', 'due_date', 'lines'],
        },
    },
    {
        name: 'get_account_balance',
        description: 'Get the balance of an account as of a specific date',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID',
                },
                account_id: {
                    type: 'string',
                    description: 'The account ID to get balance for',
                },
                as_of_date: {
                    type: 'string',
                    description: 'Date to calculate balance as of (YYYY-MM-DD)',
                },
            },
            required: ['tenant_id', 'account_id'],
        },
    },
    {
        name: 'generate_report',
        description: 'Generate a financial report',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID',
                },
                report_type: {
                    type: 'string',
                    enum: ['trial_balance', 'balance_sheet', 'income_statement', 'cash_flow'],
                    description: 'Type of report to generate',
                },
                start_date: {
                    type: 'string',
                    description: 'Start date for the report period (YYYY-MM-DD)',
                },
                end_date: {
                    type: 'string',
                    description: 'End date for the report period (YYYY-MM-DD)',
                },
            },
            required: ['tenant_id', 'report_type'],
        },
    },
    {
        name: 'list_contacts',
        description: 'List contacts (customers and vendors)',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID',
                },
                search: {
                    type: 'string',
                    description: 'Search term to filter contacts by name',
                },
                type: {
                    type: 'string',
                    enum: ['CUSTOMER', 'VENDOR', 'BOTH'],
                    description: 'Filter by contact type',
                },
            },
            required: ['tenant_id'],
        },
    },
    {
        name: 'record_payment',
        description: 'Record a payment received or made',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID',
                },
                payment_type: {
                    type: 'string',
                    enum: ['RECEIVED', 'MADE'],
                    description: 'Type of payment',
                },
                amount: {
                    type: 'string',
                    description: 'Payment amount',
                },
                payment_date: {
                    type: 'string',
                    description: 'Date of payment (YYYY-MM-DD)',
                },
                contact_id: {
                    type: 'string',
                    description: 'Contact ID (optional)',
                },
                invoice_id: {
                    type: 'string',
                    description: 'Invoice ID to allocate payment to (optional)',
                },
            },
            required: ['tenant_id', 'payment_type', 'amount', 'payment_date'],
        },
    },
    {
        name: 'get_chart_of_accounts',
        description: 'Get the chart of accounts for a tenant',
        inputSchema: {
            type: 'object',
            properties: {
                tenant_id: {
                    type: 'string',
                    description: 'The tenant ID',
                },
            },
            required: ['tenant_id'],
        },
    },
    {
        name: 'reset_demo_data',
        description: 'Reset demo data (only works for demo tenants)',
        inputSchema: {
            type: 'object',
            properties: {
                secret: {
                    type: 'string',
                    description: 'Demo reset secret key',
                },
            },
            required: ['secret'],
        },
    },
];
// API helper function
async function apiRequest(method, path, body) {
    const url = `${API_BASE}${path}`;
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_TOKEN) {
        headers['Authorization'] = `Bearer ${API_TOKEN}`;
    }
    const response = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(`API error (${response.status}): ${error}`);
    }
    return response.json();
}
// Handle tool calls
export async function handleToolCall(name, args) {
    try {
        let result;
        switch (name) {
            case 'list_invoices': {
                const { tenant_id, status, type, limit } = args;
                const params = new URLSearchParams();
                if (status)
                    params.set('status', status);
                if (type)
                    params.set('type', type);
                if (limit)
                    params.set('limit', String(limit));
                const query = params.toString() ? `?${params.toString()}` : '';
                result = await apiRequest('GET', `/api/v1/tenants/${tenant_id}/invoices${query}`);
                break;
            }
            case 'create_invoice': {
                const { tenant_id, ...invoiceData } = args;
                result = await apiRequest('POST', `/api/v1/tenants/${tenant_id}/invoices`, invoiceData);
                break;
            }
            case 'get_account_balance': {
                const { tenant_id, account_id, as_of_date } = args;
                const params = as_of_date ? `?as_of=${as_of_date}` : '';
                result = await apiRequest('GET', `/api/v1/tenants/${tenant_id}/reports/account-balance/${account_id}${params}`);
                break;
            }
            case 'generate_report': {
                const { tenant_id, report_type, start_date, end_date } = args;
                const params = new URLSearchParams();
                if (start_date)
                    params.set('start_date', start_date);
                if (end_date)
                    params.set('end_date', end_date);
                const query = params.toString() ? `?${params.toString()}` : '';
                result = await apiRequest('GET', `/api/v1/tenants/${tenant_id}/reports/${report_type.replace('_', '-')}${query}`);
                break;
            }
            case 'list_contacts': {
                const { tenant_id, search, type } = args;
                const params = new URLSearchParams();
                if (search)
                    params.set('search', search);
                if (type)
                    params.set('type', type);
                const query = params.toString() ? `?${params.toString()}` : '';
                result = await apiRequest('GET', `/api/v1/tenants/${tenant_id}/contacts${query}`);
                break;
            }
            case 'record_payment': {
                const { tenant_id, ...paymentData } = args;
                result = await apiRequest('POST', `/api/v1/tenants/${tenant_id}/payments`, paymentData);
                break;
            }
            case 'get_chart_of_accounts': {
                const { tenant_id } = args;
                result = await apiRequest('GET', `/api/v1/tenants/${tenant_id}/accounts`);
                break;
            }
            case 'reset_demo_data': {
                const { secret } = args;
                result = await apiRequest('POST', `/api/demo/reset?secret=${secret}`);
                break;
            }
            default:
                throw new Error(`Unknown tool: ${name}`);
        }
        return {
            content: [
                {
                    type: 'text',
                    text: JSON.stringify(result, null, 2),
                },
            ],
        };
    }
    catch (error) {
        return {
            content: [
                {
                    type: 'text',
                    text: `Error: ${error instanceof Error ? error.message : String(error)}`,
                },
            ],
        };
    }
}
