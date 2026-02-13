import chalk from 'chalk';
import ora, { Ora } from 'ora';
import inquirer from 'inquirer';
import fs from 'fs-extra';
import path from 'path';
import open from 'open';
import commandExists from 'command-exists';
import { readConfig } from '../services/config';

interface IssueData {
  title: string;
  body: string;
  labels?: string[];
}

export async function pushCommand(options: {
  file?: string;
  platform?: string;
  repo?: string;
  dryRun?: boolean;
}) {
  const spinner = ora({ color: 'cyan' });

  try {
    const config = await readConfig();
    if (!config) {
      console.error(chalk.red.bold('\n❌ Not initialized'));
      console.error(chalk.red('Run `cortex-eval init` first\n'));
      process.exit(1);
    }

    const crFile = options.file || 'cr-latest.md';
    
    spinner.start('Reading CR file...');
    const crPath = path.resolve(crFile);
    
    if (!await fs.pathExists(crPath)) {
      spinner.fail();
      console.error(chalk.red(`CR file not found: ${crPath}`));
      console.error(chalk.gray('Make sure the file exists or specify a different path with --file'));
      process.exit(1);
    }
    
    const crContent = await fs.readFile(crPath, 'utf-8');
    spinner.succeed();

    spinner.start('Parsing CR...');
    const issueData = parseCR(crContent);
    spinner.succeed();

    console.log(chalk.bold('\n📝 Parsed Issue\n'));
    console.log(chalk.gray('Title:'), chalk.white(issueData.title));
    console.log(chalk.gray('Body length:'), chalk.cyan(issueData.body.length.toString()), 'characters');

    if (!options.platform) {
      const answers = await inquirer.prompt([
        {
          type: 'list',
          name: 'platform',
          message: 'Select platform:',
          choices: [
            { name: 'GitHub', value: 'github' },
            { name: 'Jira', value: 'jira' },
            { name: 'Linear', value: 'linear' },
          ],
        },
      ]);
      options.platform = answers.platform;
    }

    if (options.dryRun) {
      console.log(chalk.yellow.bold('\n🔍 Dry Run Mode\n'));
      console.log(chalk.gray('Platform:'), chalk.cyan(options.platform));
      console.log(chalk.gray('Title:'), chalk.white(issueData.title));
      console.log(chalk.gray('Body:'));
      console.log(chalk.gray('─'.repeat(80)));
      console.log(issueData.body);
      console.log(chalk.gray('─'.repeat(80)));
      console.log(chalk.green.bold('\n✅ Dry run complete - no issue created'));
      return;
    }

    switch (options.platform) {
      case 'github':
        await pushToGitHub(issueData, options.repo);
        break;
      case 'jira':
        await pushToJira(issueData);
        break;
      case 'linear':
        await pushToLinear(issueData);
        break;
      default:
        console.error(chalk.red(`Unknown platform: ${options.platform}`));
        process.exit(1);
    }

  } catch (error) {
    spinner.stop();
    const err = error as Error;
    console.error(chalk.red.bold('\n❌ Push failed:'), chalk.red(err.message));
    process.exit(1);
  }
}

function parseCR(content: string): IssueData {
  const lines = content.split('\n');
  let title = '';
  let body = '';
  let inBody = false;
  let labels: string[] = [];

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    
    if (!inBody) {
      if (line.startsWith('# ')) {
        title = line.substring(2).trim();
        inBody = true;
      } else if (line.startsWith('**Title:**') || line.startsWith('Title:')) {
        title = line.split(':', 2)[1]?.trim() || '';
      }
    } else {
      body += line + '\n';
    }

    if (line.startsWith('**Labels:**') || line.startsWith('Labels:')) {
      const labelsStr = line.split(':', 2)[1]?.trim() || '';
      labels = labelsStr.split(',').map(l => l.trim()).filter(l => l);
    }
  }

  if (!title) {
    title = content.split('\n')[0]?.substring(0, 100) || 'Untitled Issue';
  }

  if (!body || body.trim() === '') {
    body = content;
  }

  return { title, body: body.trim(), labels };
}

async function pushToGitHub(issueData: IssueData, repo?: string) {
  let repoPath = repo;

  if (!repoPath) {
    try {
      repoPath = await getGitRemote();
    } catch (error) {
      console.error(chalk.yellow('\n⚠️  Could not detect GitHub repository'));
      console.error(chalk.gray('Specify with --repo owner/repo\n'));
      process.exit(1);
    }
  }

  if (!commandExists.sync('gh')) {
    console.error(chalk.red.bold('\n❌ GitHub CLI (gh) not found'));
    console.error(chalk.gray('Install: https://cli.github.com/\n'));
    process.exit(1);
  }

  const { spawn } = await import('child_process');
  const spinner = ora({ color: 'cyan' });

  spinner.start('Creating GitHub issue...');
  
  try {
    const result = await new Promise<string>((resolve, reject) => {
      const args = [
        'issue',
        'create',
        '--repo', repoPath,
        '--title', issueData.title,
        '--body', issueData.body,
      ];
      
      if (issueData.labels && issueData.labels.length > 0) {
        args.push('--label', issueData.labels.join(','));
      }

      const proc = spawn('gh', args, { stdio: ['pipe', 'pipe', 'pipe'] });
      
      let output = '';
      let errorOutput = '';

      proc.stdout.on('data', (data) => {
        output += data.toString();
      });

      proc.stderr.on('data', (data) => {
        errorOutput += data.toString();
      });

      proc.on('close', (code) => {
        if (code === 0) {
          resolve(output.trim());
        } else {
          reject(new Error(errorOutput || 'GitHub CLI failed'));
        }
      });
    });

    spinner.succeed();

    const urlMatch = result.match(/https?:\/\/[^\s]+/);
    const issueUrl = urlMatch ? urlMatch[0] : result;

    console.log(chalk.green.bold('\n✅ Issue created successfully!'));
    console.log(chalk.gray('URL:'), chalk.cyan(issueUrl));

    if (issueUrl.startsWith('http')) {
      const answers = await inquirer.prompt([
        {
          type: 'confirm',
          name: 'open',
          message: 'Open issue in browser?',
          default: true,
        },
      ]);

      if (answers.open) {
        await open(issueUrl);
      }
    }

  } catch (error) {
    spinner.fail();
    throw error;
  }
}

async function pushToJira(issueData: IssueData) {
  console.log(chalk.yellow.bold('\n⚠️  Jira Integration'));
  console.log(chalk.gray('Jira integration requires configuration.\n'));
  console.log(chalk.gray('To set up Jira:'));
  console.log(chalk.gray('1. Create Jira API token: https://id.atlassian.com/manage-profile/security/api-tokens'));
  console.log(chalk.gray('2. Configure environment variables:'));
  console.log(chalk.gray('   - JIRA_BASE_URL (e.g., https://yourcompany.atlassian.net)'));
  console.log(chalk.gray('   - JIRA_EMAIL (your email)'));
  console.log(chalk.gray('   - JIRA_API_TOKEN\n'));
  console.log(chalk.gray('Then run: cortex-eval push --platform jira'));
}

async function pushToLinear(issueData: IssueData) {
  console.log(chalk.yellow.bold('\n⚠️  Linear Integration'));
  console.log(chalk.gray('Linear integration requires configuration.\n'));
  console.log(chalk.gray('To set up Linear:'));
  console.log(chalk.gray('1. Get API key: https://linear.app/settings/api'));
  console.log(chalk.gray('2. Set environment variable: LINEAR_API_KEY\n'));
  console.log(chalk.gray('Then run: cortex-eval push --platform linear'));
}

async function getGitRemote(): Promise<string> {
  const { execSync } = await import('child_process');
  
  try {
    const output = execSync('git remote get-url origin', { encoding: 'utf-8' }).trim();
    
    const match = output.match(/github\.com[\/:]([^\/]+)\/([^\/\.]+)/);
    if (match) {
      return `${match[1]}/${match[2]}`;
    }
    
    throw new Error('Could not parse remote URL');
  } catch (error) {
    throw new Error('Not in a git repository or no remote configured');
  }
}
