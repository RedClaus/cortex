import chalk from 'chalk';
import ora, { Ora } from 'ora';
import fs from 'fs-extra';
import { apiClient } from '../services/api';
import { readConfig } from '../services/config';

interface ComparisonResult {
  input: string;
  valueScore: number;
  executiveSummary: string;
  technicalFeasibility: string;
  gapAnalysis: string;
  suggestedCR: string;
  providerUsed: string;
}

export async function compareCommand(
  inputs: string[],
  options: {
    criteria?: string;
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

    const criteria = options.criteria ? options.criteria.split(',').map(c => c.trim()) : undefined;

    console.log(chalk.bold('\n📊 Comparing', chalk.cyan(inputs.length.toString()), 'inputs...\n'));

    const results: ComparisonResult[] = [];
    
    for (let i = 0; i < inputs.length; i++) {
      const input = inputs[i];
      spinner.start(`Analyzing ${chalk.cyan(input)} (${i + 1}/${inputs.length})...`);
      
      try {
        const result = await apiClient.analyzeEvaluation({
          codebaseId,
          inputType: 'snippet',
          inputContent: input,
        });
        
        results.push({
          input,
          valueScore: result.valueScore,
          executiveSummary: result.executiveSummary,
          technicalFeasibility: result.technicalFeasibility,
          gapAnalysis: result.gapAnalysis,
          suggestedCR: result.suggestedCR,
          providerUsed: result.providerUsed,
        });
        
        spinner.succeed(chalk.green(`✓ ${input} (${result.valueScore}/100)`));
      } catch (error) {
        spinner.fail(chalk.red(`✗ Failed: ${(error as Error).message}`));
      }
    }

    displayComparison(results, criteria);

    if (options.output) {
      await saveComparison(results, options.output, criteria);
      console.log(chalk.green.bold('\n✅ Comparison saved to:'), chalk.cyan(options.output));
    }

  } catch (error) {
    spinner.stop();
    const err = error as Error;
    console.error(chalk.red.bold('\n❌ Comparison failed:'), chalk.red(err.message));
    process.exit(1);
  }
}

function displayComparison(results: ComparisonResult[], criteria?: string[]) {
  console.log(chalk.bold('\n📊 Comparison Results\n'));

  results.sort((a, b) => b.valueScore - a.valueScore);

  results.forEach((result, index) => {
    const scoreColor = result.valueScore >= 80 ? chalk.green :
                       result.valueScore >= 60 ? chalk.yellow :
                       chalk.red;
    
    const rankIcon = index === 0 ? '🥇' : index === 1 ? '🥈' : index === 2 ? '🥉' : '  ';
    
    console.log(`${rankIcon} ${chalk.bold(result.input.substring(0, 60))}${result.input.length > 60 ? '...' : ''}`);
    console.log(`   ${chalk.gray('Score:')} ${scoreColor(result.valueScore + '/100')} ${chalk.gray(`| Provider: ${result.providerUsed}`)}`);
    console.log(`   ${chalk.gray('Summary:')} ${chalk.white(result.executiveSummary.substring(0, 100))}...\n`);
  });

  if (criteria) {
    console.log(chalk.bold('\n📋 Evaluation Criteria'));
    criteria.forEach((criterion, index) => {
      console.log(`  ${index + 1}. ${chalk.cyan(criterion)}`);
    });
  }

  const bestResult = results[0];
  if (bestResult) {
    console.log(chalk.bold('\n🏆 Recommended Approach'));
    console.log(chalk.gray('Input:'), chalk.cyan(bestResult.input));
    console.log(chalk.gray('Score:'), chalk.green.bold(bestResult.valueScore + '/100'));
    console.log(chalk.gray('Rationale:'), chalk.white(bestResult.executiveSummary));
  }

  console.log(chalk.bold('\n📈 Comparison Matrix'));
  console.log(chalk.gray('─'.repeat(80)));
  
  const header = `${chalk.bold('Approach')}${' '.repeat(20 - Math.min(20, bestResult?.input.length || 0))} ${chalk.bold('Score')} ${chalk.bold('Feasibility')} ${chalk.bold('Complexity')}`;
  console.log(header);
  console.log(chalk.gray('─'.repeat(80)));

  results.forEach((result) => {
    const complexity = estimateComplexity(result);
    const name = result.input.substring(0, 20);
    console.log(`${name.padEnd(20)} ${chalk.white(result.valueScore.toString().padStart(3))} ${chalk.white(result.technicalFeasibility.substring(0, 10))} ${complexity}`);
  });
}

function estimateComplexity(result: ComparisonResult): string {
  const score = result.valueScore;
  const feasibility = result.technicalFeasibility.toLowerCase();
  
  if (feasibility.includes('complex') || feasibility.includes('challenging')) {
    return chalk.yellow('High');
  } else if (feasibility.includes('simple') || feasibility.includes('easy')) {
    return chalk.green('Low');
  } else if (score > 80) {
    return chalk.green('Low');
  } else if (score < 50) {
    return chalk.red('High');
  }
  return chalk.white('Medium');
}

async function saveComparison(
  results: ComparisonResult[],
  outputPath: string,
  criteria?: string[]
) {
  const timestamp = new Date().toISOString();
  
  let markdown = `# Comparison Results\n\n**Generated:** ${timestamp}\n**Inputs:** ${results.length}\n\n`;
  
  if (criteria && criteria.length > 0) {
    markdown += `## Evaluation Criteria\n\n`;
    criteria.forEach((criterion, index) => {
      markdown += `${index + 1}. ${criterion}\n`;
    });
    markdown += '\n';
  }

  markdown += `## Comparison Matrix\n\n`;
  markdown += `| Approach | Score | Feasibility | Complexity |\n`;
  markdown += `|----------|-------|-------------|-------------|\n`;
  
  results.sort((a, b) => b.valueScore - a.valueScore).forEach((result) => {
    const name = result.input.substring(0, 30).replace(/\|/g, '\\|');
    const feasibility = result.technicalFeasibility.substring(0, 20);
    const complexity = estimateComplexity(result);
    markdown += `| ${name} | ${result.valueScore}/100 | ${feasibility} | ${complexity} |\n`;
  });

  markdown += '\n## Detailed Analysis\n\n';

  results.forEach((result, index) => {
    markdown += `### ${index + 1}. ${result.input}\n\n`;
    markdown += `- **Value Score:** ${result.valueScore}/100\n`;
    markdown += `- **Provider:** ${result.providerUsed}\n\n`;
    markdown += `**Executive Summary:**\n${result.executiveSummary}\n\n`;
    markdown += `**Technical Feasibility:**\n${result.technicalFeasibility}\n\n`;
    markdown += `**Gap Analysis:**\n${result.gapAnalysis}\n\n`;
    markdown += `---\n\n`;
  });

  markdown += `## Recommendation\n\n`;
  
  const bestResult = results[0];
  if (bestResult) {
    markdown += `**Recommended:** ${bestResult.input}\n\n`;
    markdown += `**Value Score:** ${bestResult.valueScore}/100\n\n`;
    markdown += `**Rationale:**\n${bestResult.executiveSummary}\n\n`;
  }

  await fs.writeFile(outputPath, markdown, 'utf-8');
}
