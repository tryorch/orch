// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	integrations: [
		starlight({
			title: 'Orch',
			description:
				'Ephemeral environment orchestration for preview, test, and development sandboxes.',
			customCss: ['./src/styles/custom.css'],
			logo: {
				src: './src/assets/logo.svg',
			},
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/orch-io/orch' }],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'getting-started/introduction' },
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Quickstart', slug: 'getting-started/quickstart' },
					],
				},
				{
					label: 'Learn',
					items: [
						{ label: 'Environments', slug: 'concepts/environments' },
						{ label: 'Components', slug: 'concepts/components' },
						{ label: 'Runners', slug: 'concepts/runners' },
						{ label: 'State and Recovery', slug: 'concepts/state-and-recovery' },
						{ label: 'Lifecycle Hooks', slug: 'concepts/lifecycle-hooks' },
						{ label: 'Outputs', slug: 'concepts/outputs' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Local Script Environment', slug: 'guides/local-script' },
						{ label: 'Docker Compose Environment', slug: 'guides/docker-compose' },
						{ label: 'Terraform Environment', slug: 'guides/terraform' },
						{ label: 'CI Environments', slug: 'guides/ci-environments' },
					],
				},
				{
					label: 'Reference',
					items: [
						{
							label: 'Manifest',
							collapsed: false,
							items: [
								{ label: 'Overview', slug: 'reference/manifest' },
								{ label: 'Metadata', slug: 'reference/manifest/metadata' },
								{ label: 'Inputs', slug: 'reference/manifest/inputs' },
								{ label: 'State', slug: 'reference/manifest/state' },
								{ label: 'Runners', slug: 'reference/manifest/runners' },
								{ label: 'Components', slug: 'reference/manifest/components' },
								{ label: 'Source and Files', slug: 'reference/manifest/source-and-files' },
								{ label: 'Env', slug: 'reference/manifest/env' },
								{ label: 'Outputs', slug: 'reference/manifest/outputs' },
								{ label: 'Hooks', slug: 'reference/manifest/hooks' },
							],
						},
						{ label: 'CLI', slug: 'reference/cli' },
						{ label: 'State Backends', slug: 'reference/state-backends' },
					],
				},
				{
					label: 'Adapters',
					items: [
						{ label: 'Script', slug: 'adapters/script' },
						{ label: 'Docker Compose', slug: 'adapters/docker-compose' },
						{ label: 'Terraform', slug: 'adapters/terraform' },
						{ label: 'CloudFormation', slug: 'adapters/cloudformation' },
					],
				},
				{
					label: 'Security',
					items: [
						{ label: 'Overview', slug: 'security/overview' },
						{ label: 'SSH', slug: 'security/ssh' },
						{ label: 'State Security', slug: 'security/state-security' },
					],
				},
			],
		}),
	],
});
