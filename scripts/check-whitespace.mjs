import fs from 'fs';

const mode = process.argv[2];
const files = process.argv.slice(3);

if (!mode || files.length === 0) {
  console.error(
    'usage: node scripts/check-whitespace.mjs <trailing-whitespace|end-of-file-newline> <files...>'
  );
  process.exit(2);
}

let failed = false;

for (const file of files) {
  const content = fs.readFileSync(file, 'utf8');

  if (mode === 'trailing-whitespace') {
    const lines = content.split(/\r?\n/);

    for (let index = 0; index < lines.length; index += 1) {
      if (/[ \t]+$/.test(lines[index])) {
        console.error(`${file}:${index + 1}: trailing whitespace`);
        failed = true;
      }
    }
    continue;
  }

  if (mode === 'end-of-file-newline') {
    if (content.length > 0 && !content.endsWith('\n')) {
      console.error(`${file}: missing newline at end of file`);
      failed = true;
    }
    continue;
  }

  console.error(`unknown mode: ${mode}`);
  process.exit(2);
}

if (failed) {
  process.exit(1);
}
