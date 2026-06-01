import { readFileSync } from 'node:fs';
import { join } from 'node:path';

export const prerender = true;

export function GET() {
	const script = readFileSync(join(process.cwd(), '../scripts/install.sh'), 'utf8');

	return new Response(script, {
		headers: {
			'content-type': 'text/x-shellscript; charset=utf-8',
			'cache-control': 'public, max-age=300',
		},
	});
}
