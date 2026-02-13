import inquirer from 'inquirer';
import chalk from 'chalk';
import path from 'path';
import { writeConfig, generateProjectId, CortexEvalConfig } from '../services/config';

export async function initCommand(options: { name?: string; dir?: string }) {
  const workingDir = options.dir || process.cwd();
  
  console.log(chalk.blue.bold('\n🚀 Initializing Cortex Evaluator Project\n'));

  try {
    const answers = await inquirer.prompt([
      {
        type: 'input',
        name: 'projectName',
        message: 'Project name:',
        default: options.name || path.basename(workingDir),
        validate: (input: string) => {
          if (!input.trim()) {
            return 'Project name is required';
          }
          return true;
        },
      },
      {
        type: 'input',
        name: 'apiBaseUrl',
        message: 'Backend API URL:',
        default: 'http://localhost:8000',
        validate: (input: string) => {
          if (!input.trim()) {
            return 'API URL is required';
          }
          try {
            new URL(input);
            return true;
          } catch {
            return 'Please enter a valid URL (e.g., http://localhost:8000)';
          }
        },
      },
      {
        type: 'confirm',
        name: 'shouldInitCodebase',
        message: 'Initialize codebase now?',
        default: true,
      },
    ]);

    const config: CortexEvalConfig = {
      projectId: generateProjectId(),
      projectName: answers.projectName,
      apiBaseUrl: answers.apiBaseUrl,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    await writeConfig(config, workingDir);

    console.log(chalk.green.bold('\n✅ Project initialized successfully!\n'));
    console.log(chalk.gray('Project ID:'), chalk.cyan(config.projectId));
    console.log(chalk.gray('Config file:'), chalk.cyan(path.join(workingDir, '.cortex-eval.json')));

    if (answers.shouldInitCodebase) {
      console.log(chalk.yellow('\n💡 Next step: Initialize your codebase'));
      console.log(chalk.gray('  Run: cortex-eval init-codebase <local-path|github-url>'));
    } else {
      console.log(chalk.yellow('\n💡 Initialize your codebase when ready:'));
      console.log(chalk.gray('  Run: cortex-eval init-codebase <local-path|github-url>'));
    }
  } catch (error) {
    if (error instanceof Error) {
      console.error(chalk.red.bold('\n❌ Initialization failed:'), chalk.red(error.message));
      process.exit(1);
    }
  }
}
