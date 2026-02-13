import path from 'path';
import chalk from 'chalk';
import ora, { Ora } from 'ora';
import fs from 'fs-extra';
import { apiClient } from '../services/api';
import { readConfig, updateConfig } from '../services/config';

export async function analyzeCommand(
  input: string,
  options: {
    provider?: string;
    template?: string;
    output?: string;
    codebase?: string;
  }
) {
  const spinner = ora({ color: 'cyan' });

  try {
    const config = await readConfig();
    if (!config) {
      console.error(chalk.red.bold('\n❌ Not initialized'));
      console.error(chalk.red('Run `cortex-eval init` first\n'));
      process.exit(1);
    }

    if (config.apiBaseUrl) {
      apiClient.setBaseURL(config.apiBaseUrl);
    }

    const codebaseId = options.codebase || config.codebaseId;
    if (!codebaseId) {
      console.error(chalk.red.bold('\n❌ No codebase initialized'));
      console.error(chalk.red('Run `cortex-eval init-codebase <path>` first\n'));
      process.exit(1);
    }

    spinner.start('Detecting input type...');
    const inputType = detectInputType(input);
    spinner.succeed(chalk.green(`Input type: ${chalk.bold(inputType)}`));

    let inputContent = input;
    if (inputType === 'file') {
      spinner.start('Reading file...');
      if (!await fs.pathExists(input)) {
        spinner.fail();
        console.error(chalk.red(`File not found: ${input}`));
        process.exit(1);
      }
      inputContent = await fs.readFile(input, 'utf-8');
      spinner.succeed();
    } else if (inputType === 'arxiv-url') {
      const paperId = extractArxivId(input);
      if (paperId) {
        inputContent = paperId;
      }
    }

    spinner.start('Analyzing against codebase...');
    const normalizedInputType = normalizeInputType(inputType);
    const result = await apiClient.analyzeEvaluation({
      codebaseId,
      inputType: normalizedInputType,
      inputContent,
      providerPreference: options.provider,
    });
    spinner.succeed();

    displayResults(result);

    const outputFilePath = options.output || `cr-${Date.now()}.md`;
    await saveCR(result.suggestedCR, outputFilePath, result);
    console.log(chalk.green.bold('\n✅ CR saved to:'), chalk.cyan(outputFilePath));

  } catch (error) {
    spinner.stop();
    const err = error as Error;
    console.error(chalk.red.bold('\n❌ Analysis failed:'), chalk.red(err.message));
    if ((error as any).status) {
      console.error(chalk.gray(`Status: ${(error as any).status}`));
    }
    process.exit(1);
  }
}

function detectInputType(input: string): 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url' | 'file' {
  if (input.startsWith('https://arxiv.org/abs/') || input.startsWith('http://arxiv.org/abs/')) {
    return 'arxiv-url' as any;
  }
  if (/^\d{4}\.\d+$/.test(input) || /^\d{5}$/.test(input)) {
    return 'arxiv';
  }
  if (input.startsWith('http://') || input.startsWith('https://')) {
    return 'url';
  }
  if (input.endsWith('.pdf')) {
    return 'pdf';
  }
  if (input.startsWith('github.com/') || input.includes('github.com/')) {
    return 'repo';
  }
  return 'snippet';
}

function extractArxivId(url: string): string | null {
  const match = url.match(/arxiv\.org\/abs\/(\d+\.\d+)/);
  return match ? match[1] : null;
}

function normalizeInputType(inputType: string): 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url' {
  if (inputType === 'arxiv-url' || inputType === 'file') {
    return 'arxiv';
  }
  return inputType as 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url';
}

function displayResults(result: any) {
  console.log(chalk.bold('\n📊 Analysis Results\n'));
  
  const scoreColor = result.valueScore >= 80 ? chalk.green.bold :
                     result.valueScore >= 60 ? chalk.yellow.bold :
                     chalk.red.bold;
  
  console.log(chalk.gray('Value Score:'), scoreColor(`${result.valueScore}/100`));
  console.log(chalk.gray('Provider:'), chalk.cyan(result.providerUsed));
  
  console.log(chalk.bold('\n📝 Executive Summary'));
  console.log(result.executiveSummary);
  
  console.log(chalk.bold('\n🔧 Technical Feasibility'));
  console.log(result.technicalFeasibility);
  
  console.log(chalk.bold('\n📈 Gap Analysis'));
  console.log(result.gapAnalysis);
  
  if (result.similarEvaluations && result.similarEvaluations.length > 0) {
    console.log(chalk.bold('\n🔍 Similar Evaluations'));
    result.similarEvaluations.slice(0, 3).forEach((evaluation: any, index: number) => {
      console.log(chalk.gray(`  ${index + 1}.`), chalk.cyan(evaluation.id), chalk.gray(`(score: ${evaluation.score.toFixed(2)})`));
    });
  }
}

async function saveCR(cr: string, filePath: string, result: any) {
  const timestamp = new Date().toISOString();
  const header = `# Change Request\n\n**Generated:** ${timestamp}\n**Value Score:** ${result.valueScore}/100\n**Provider:** ${result.providerUsed}\n\n---\n\n`;
  await fs.writeFile(filePath, header + cr, 'utf-8');
}
