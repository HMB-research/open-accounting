import { Tool } from '@modelcontextprotocol/sdk/types.js';
export declare const tools: Tool[];
export declare function handleToolCall(name: string, args: Record<string, unknown> | undefined): Promise<{
    content: Array<{
        type: string;
        text: string;
    }>;
}>;
