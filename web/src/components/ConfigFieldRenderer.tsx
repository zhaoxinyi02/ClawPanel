import { Check } from 'lucide-react';
import type { ReactNode } from 'react';

export type SharedConfigField = {
  key: string;
  label: string;
  type: 'text' | 'password' | 'toggle' | 'number' | 'select' | 'textarea';
  options?: string[];
  placeholder?: string;
  help?: string;
  rows?: number;
};

type Props = {
  field: SharedConfigField;
  value: any;
  textareaValue?: string;
  hasExplicitValue: boolean;
  onChange: (value: string) => void;
  onToggle?: () => void;
  fullWidth?: boolean;
  fieldHelp?: string;
  emptyOptionLabel?: string;
  compactToggle?: boolean;
  toggleStateLabel?: string;
  accent?: 'violet' | 'blue';
  renderFooter?: ReactNode;
};

export default function ConfigFieldRenderer({
  field,
  value,
  textareaValue = '',
  hasExplicitValue,
  onChange,
  onToggle,
  fullWidth,
  fieldHelp,
  emptyOptionLabel,
  compactToggle,
  toggleStateLabel,
  accent = 'violet',
  renderFooter,
}: Props) {
  const accentClass = accent === 'blue'
    ? {
        ring: 'focus:ring-blue-100 dark:focus:ring-blue-900/30 focus:border-blue-500',
        toggle: value ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600',
        toggleFocus: 'focus:ring-blue-500',
        text: value ? 'text-blue-600 dark:text-blue-400 font-medium' : 'text-gray-500',
      }
    : {
        ring: 'focus:ring-violet-100 dark:focus:ring-violet-900/30 focus:border-violet-500',
        toggle: value ? 'bg-violet-600' : 'bg-gray-300 dark:bg-gray-600',
        toggleFocus: 'focus:ring-violet-500',
        text: value ? 'text-violet-600 dark:text-violet-400 font-medium' : 'text-gray-500',
      };

  return (
    <div key={field.key} className={fullWidth || field.type === 'textarea' ? 'md:col-span-2' : ''}>
      {field.type !== 'toggle' && (
        <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">
          {field.label}
        </label>
      )}

      {field.type === 'toggle' ? (
        <div
          className={`rounded-lg border transition-colors ${compactToggle
            ? 'flex items-start justify-between gap-4 px-4 py-3 border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 h-full'
            : 'flex items-center gap-3 p-3 border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30'}`}
        >
          <div className="min-w-0 flex-1">
            <div className="text-sm font-semibold text-gray-900 dark:text-white">{field.label}</div>
            {fieldHelp && <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{fieldHelp}</p>}
          </div>
          <button
            type="button"
            onClick={onToggle}
            className={`relative shrink-0 w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 ${accentClass.toggleFocus} ${accentClass.toggle}`}
          >
            <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${value ? 'translate-x-4' : ''}`} />
          </button>
          {toggleStateLabel && <span className={`text-xs ${accentClass.text}`}>{toggleStateLabel}</span>}
        </div>
      ) : field.type === 'select' ? (
        <div className="relative">
          <select
            name={field.key}
            value={value ?? ''}
            onChange={e => onChange(e.target.value)}
            className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 outline-none ${accentClass.ring}
              ${hasExplicitValue ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100' : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
          >
            <option value="">{emptyOptionLabel || field.placeholder || 'Not configured'}</option>
            {field.options?.map(opt => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        </div>
      ) : field.type === 'textarea' ? (
        <textarea
          name={field.key}
          rows={field.rows || 3}
          value={textareaValue}
          onChange={e => onChange(e.target.value)}
          placeholder={field.placeholder || 'Not configured'}
          className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 outline-none resize-y ${accentClass.ring}
            ${hasExplicitValue ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100' : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
        />
      ) : (
        <div className="relative">
          <input
            name={field.key}
            type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
            value={value ?? ''}
            onChange={e => onChange(e.target.value)}
            placeholder={field.placeholder || 'Not configured'}
            className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 outline-none pr-8 ${accentClass.ring}
              ${hasExplicitValue ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100' : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
          />
          {hasExplicitValue && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 text-emerald-500">
              <Check size={14} strokeWidth={3} />
            </div>
          )}
        </div>
      )}

      {field.type !== 'toggle' && fieldHelp && (
        <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{fieldHelp}</p>
      )}
      {renderFooter}
    </div>
  );
}
