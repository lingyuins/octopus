import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const create = readFileSync(new URL('./Create.tsx', import.meta.url), 'utf8');
const form = readFileSync(new URL('./Form.tsx', import.meta.url), 'utf8');
const toolbar = readFileSync(new URL('../toolbar/index.tsx', import.meta.url), 'utf8');
const dialog = readFileSync(new URL('../../ui/morphing-dialog.tsx', import.meta.url), 'utf8');

test('channel creation starts with the form and allows presets on every viewport', () => {
  assert.match(create, /\[showPresetPicker, setShowPresetPicker\] = useState\(false\)/);
  assert.match(create, /onClick=\{\(\) => setShowPresetPicker\(!showPresetPicker\)\}/);
  assert.doesNotMatch(create, /!isMobile && showPresetPicker/);
  assert.match(create, /layout="create"/);
  assert.match(create, /showTemplatePicker=\{false\}/);
  assert.match(create, /cancelText=\{tForm\('modelPicker.cancel'\)\}/);
});

test('channel dialog overrides default width and height constraints without nested decoration', () => {
  assert.match(toolbar, /if \(activeItem === 'channel'\)[\s\S]*?sm:max-w-none/);
  assert.match(toolbar, /max-h-\[calc\(100dvh-1rem\)\]/);
  assert.match(toolbar, /md:w-\[min\(100vw-3rem,64rem\)\]/);
  assert.doesNotMatch(create, /radial-gradient|shadow-inner|tracking-tight/);
  assert.match(create, /disableLayoutAnimation className="flex min-h-0 flex-1 flex-col overflow-hidden"/);
});

test('channel creation uses the independent dialog exit lifecycle and closes before any form reset', () => {
  assert.match(toolbar, /<MorphingDialog disableSharedLayout=\{toolbarItem === 'channel'\}>/);
  assert.doesNotMatch(create, /resetFormData\(\)/);
  const successHandler = create.slice(create.indexOf('onSuccess: () => {'), create.indexOf('onSuccess: () => {') + 300);
  assert.match(successHandler, /setIsOpen\(false\)/);
  assert.doesNotMatch(successHandler, /setFormData|setShowPresetPicker/);
});

test('shared-layout dialogs keep the shared exit lifecycle while non-shared dialogs use explicit exit motion', () => {
  const contentProps = dialog.slice(dialog.indexOf('export type MorphingDialogContentProps'), dialog.indexOf('function MorphingDialogContent'));
  const containerProps = dialog.slice(dialog.indexOf('function MorphingDialogContainer'), dialog.indexOf('export type MorphingDialogTitleProps'));
  assert.match(dialog, /function MorphingDialogContainer\(\{ children \}: MorphingDialogContainerProps\)/);
  assert.match(containerProps, /if \(disableSharedLayout\) \{/);
  assert.match(containerProps, /exit=\{\{ opacity: 0, transition: \{ duration: 0\.12 \} \}\}/);
  assert.match(containerProps, /bg-black\/40 backdrop-blur-sm/);
  assert.match(dialog, /exit=\{disableSharedLayout \? \{ opacity: 0, scale: 0\.98 \} : undefined\}/);
  assert.doesNotMatch(contentProps, /onExitComplete/);
  assert.doesNotMatch(containerProps, /onExitComplete/);
  const title = dialog.slice(dialog.indexOf('function MorphingDialogTitle'), dialog.indexOf('export type MorphingDialogSubtitleProps'));
  const description = dialog.slice(dialog.indexOf('function MorphingDialogDescription'), dialog.indexOf('export type MorphingDialogImageProps'));
  assert.match(title, /if \(disableSharedLayout\) \{[\s\S]*?<div className=\{className\} style=\{style\}>/);
  assert.match(description, /if \(disableSharedLayout\) \{[\s\S]*?<div className=\{className\}/);
});

test('pool binding and key strategy stay inside advanced settings alongside proxy mode', () => {
  const advanced = form.slice(form.indexOf('<Accordion type="single"'), form.indexOf('</Accordion>'));
  assert.match(advanced, /t\('poolBinding'\)/);
  assert.match(advanced, /t\('keySelectionStrategy'\)/);
  assert.match(advanced, /grid min-w-0 items-start gap-4 md:grid-cols-2/);
  assert.match(advanced, /<ProxySelector\s+layout="stacked"/);
  assert.equal(form.split("t('poolBinding')").length - 1, 1);
  assert.equal(form.split("t('keySelectionStrategy')").length - 1, 1);
});

test('creation form scrolls independently and keeps safe-area-aware actions outside the scroll region', () => {
  assert.match(form, /layout = 'default'/);
  assert.match(form, /md:grid-cols-2/);
  assert.match(form, /overflow-y-auto overscroll-contain/);
  assert.match(form, /env\(safe-area-inset-bottom\)/);
  assert.match(form, /onClick=\{onCancel\}\s+disabled=\{isPending\}/);
  assert.match(form, /aria-label=\{`\$\{t\('apiKey'\)\} \$\{idx \+ 1\}`\}/);
  assert.match(form, /break-all whitespace-normal/);
  assert.doesNotMatch(form, /\[&_\[data-slot=select-trigger\]\]:w-full/);
  assert.match(form, /lg:grid-cols-\[minmax\(0,1fr\)_8rem_6\.5rem_2\.75rem\]/);
});
