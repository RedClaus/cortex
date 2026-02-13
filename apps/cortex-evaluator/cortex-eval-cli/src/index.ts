#!/usr/bin/env node

import { Command } from 'commander';
import chalk from 'chalk';
import { initCommand } from './commands/init';
import { analyzeCommand } from './commands/analyze';
import { paperCommand } from './commands/paper';
import { compareCommand } from './commands/compare';
import { pushCommand } from './commands/push';

const program = new Command();

program
  .name('cortex-eval')
  .description('CLI tool for Cortex Evaluator - Analyze papers, generate CRs, and evaluate technical proposals')
  .version('1.0.0')
  .configureHelp({
    sortSubcommands: true,
    sortOptions: true,
    styleTitle: (text: string) => chalk.bold.cyan(text),
    styleCommandText: (text: string) => chalk.yellow(text),
    styleOptionText: (text: string) => chalk.green(text),
  });

program
  .command('init')
  .description('Initialize a new Cortex Evaluator project')
  .option('--name <name>', 'Project name')
  .option('--dir <directory>', 'Project directory', '.')
  .action(initCommand);

program
  .command('analyze')
  .description('Analyze codebase against input (arxiv URL, paper ID, text, or file)')
  .argument('<input>', 'Input to analyze (arxiv URL, paper ID, text, or file path)')
  .option('--provider <provider>', 'AI provider preference (openai, anthropic, gemini, etc.)')
  .option('--template <template>', 'CR template to use')
  .option('-o, --output <file>', 'Output file for CR (default: cr-{timestamp}.md)')
  .option('--codebase <id>', 'Codebase ID to analyze against')
  .action(analyzeCommand);

program
  .command('paper')
  .description('Analyze an arXiv paper against codebase')
  .argument('<paperId>', 'arXiv paper ID (e.g., 2301.00774)')
  .option('--no-analysis', 'Only fetch paper without analysis')
  .option('--download-pdf', 'Download PDF paper')
  .option('--provider <provider>', 'AI provider preference')
  .option('--codebase <id>', 'Codebase ID to analyze against')
  .action(paperCommand);

program
  .command('compare')
  .description('Compare multiple approaches/papers side-by-side')
  .argument('<inputs...>', 'Multiple paper IDs or URLs to compare')
  .option('--criteria <criteria>', 'Comma-separated evaluation criteria')
  .option('--output <file>', 'Output file for comparison matrix')
  .option('--codebase <id>', 'Codebase ID to analyze against')
  .action(compareCommand);

program
  .command('push')
  .description('Push CR to external platform (GitHub, Jira, Linear)')
  .option('-f, --file <file>', 'CR file to push (default: cr-latest.md)')
  .option('--platform <platform>', 'Platform (github, jira, linear)')
  .option('--repo <repo>', 'Repository (for GitHub: owner/repo)')
  .option('--dry-run', 'Preview without creating issue')
  .action(pushCommand);

program.parse(process.argv);

if (!process.argv.slice(2).length) {
  program.outputHelp();
}
