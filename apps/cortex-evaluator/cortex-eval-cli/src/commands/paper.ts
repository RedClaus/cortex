import chalk from 'chalk';
import ora, { Ora } from 'ora';
import axios from 'axios';
import fs from 'fs-extra';
import { apiClient } from '../services/api';
import { readConfig } from '../services/config';

export async function paperCommand(
  paperId: string,
  options: {
    noAnalysis?: boolean;
    downloadPdf?: boolean;
    provider?: string;
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

    spinner.start('Fetching arXiv paper...');
    const paper = await apiClient.getArxivPaper(paperId);
    spinner.succeed();

    displayPaperMetadata(paper);

    if (options.downloadPdf && paper.pdfUrl) {
      spinner.start('Downloading PDF...');
      const pdfPath = await downloadPdf(paper.pdfUrl, paperId);
      spinner.succeed(chalk.green(`PDF saved to: ${pdfPath}`));
    }

    if (options.noAnalysis) {
      return;
    }

    const codebaseId = options.codebase || config.codebaseId;
    if (!codebaseId) {
      console.error(chalk.yellow.bold('\n⚠️  No codebase configured'));
      console.error(chalk.yellow('Run `cortex-eval init-codebase <path>` to analyze against your codebase\n'));
      process.exit(1);
    }

    spinner.start('Analyzing paper against codebase...');
    const result = await apiClient.analyzeEvaluation({
      codebaseId,
      inputType: 'arxiv',
      inputContent: paperId,
      providerPreference: options.provider,
    });
    spinner.succeed();

    displayResults(result);

    const outputFilePath = `cr-${paperId.replace(/\./g, '-')}-${Date.now()}.md`;
    await saveCR(result.suggestedCR, outputFilePath, result, paper);
    console.log(chalk.green.bold('\n✅ CR saved to:'), chalk.cyan(outputFilePath));

  } catch (error) {
    spinner.stop();
    const err = error as Error;
    console.error(chalk.red.bold('\n❌ Paper analysis failed:'), chalk.red(err.message));
    if ((error as any).status) {
      console.error(chalk.gray(`Status: ${(error as any).status}`));
    }
    process.exit(1);
  }
}

function displayPaperMetadata(paper: any) {
  console.log(chalk.bold('\n📄 Paper Metadata\n'));
  console.log(chalk.gray('ID:'), chalk.cyan(paper.id));
  console.log(chalk.gray('Title:'), chalk.white(paper.title));
  console.log(chalk.gray('Authors:'), chalk.white(paper.authors.join(', ')));
  console.log(chalk.gray('Published:'), chalk.white(paper.published));
  console.log(chalk.gray('Categories:'), chalk.white(paper.categories.join(', ')));
  
  if (paper.abstract) {
    console.log(chalk.bold('\n📝 Abstract'));
    console.log(chalk.white(paper.abstract.substring(0, 500)));
    if (paper.abstract.length > 500) {
      console.log(chalk.gray('...'));
    }
  }
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
}

async function saveCR(cr: string, filePath: string, result: any, paper: any) {
  const timestamp = new Date().toISOString();
  const header = `# Change Request: ${paper.title}\n\n**Generated:** ${timestamp}\n**Paper ID:** ${paper.id}\n**Value Score:** ${result.valueScore}/100\n**Provider:** ${result.providerUsed}\n\n---\n\n`;
  await fs.writeFile(filePath, header + cr, 'utf-8');
}

async function downloadPdf(pdfUrl: string, paperId: string): Promise<string> {
  const response = await axios({
    method: 'GET',
    url: pdfUrl,
    responseType: 'stream',
  });

  const filename = `paper-${paperId.replace(/\./g, '-')}.pdf`;
  const writer = fs.createWriteStream(filename);
  response.data.pipe(writer);

  return new Promise((resolve, reject) => {
    writer.on('finish', () => resolve(filename));
    writer.on('error', reject);
  });
}
