"use client";

import * as RSelect from "@radix-ui/react-select";
import * as RCheckbox from "@radix-ui/react-checkbox";
import * as RRadio from "@radix-ui/react-radio-group";
import * as RSwitch from "@radix-ui/react-switch";
import * as RLabel from "@radix-ui/react-label";
import { CalendarDays, Check, ChevronDown, ChevronUp, Clock3, Minus } from "lucide-react";
import { forwardRef, useMemo, useState, type ReactNode } from "react";
import { cn } from "../lib/utils";

/**
 * Branded form controls.
 *
 * A native <select> renders in the OPERATING SYSTEM's chrome, not ours: on
 * macOS it is a dark grey panel with system typography that ignores every token
 * in this design system, and there is no CSS that can restyle it. The same is
 * true of native checkboxes, radios and the switch we would otherwise fake.
 *
 * These are Radix primitives, which are unstyled and accessible — keyboard
 * navigation, typeahead, focus management and ARIA are handled — so all we add
 * is the token layer. As with every other component here, they read only
 * semantic and --mat-* tokens, so they follow the material and theme.
 */

// ---------------------------------------------------------------- Select

export interface SelectOption {
  value: string;
  label: string;
  hint?: string;
  disabled?: boolean;
}

export function Select({
  value,
  onValueChange,
  options,
  placeholder = "Select…",
  ariaLabel,
  className,
  disabled,
  name,
  defaultValue,
  required,
}: {
  value?: string;
  onValueChange?: (v: string) => void;
  options: readonly SelectOption[];
  placeholder?: string;
  ariaLabel?: string;
  className?: string;
  disabled?: boolean;
  name?: string;
  defaultValue?: string;
  required?: boolean;
}) {
  return (
    <RSelect.Root
      {...(value !== undefined ? { value } : {})}
      {...(onValueChange ? { onValueChange } : {})}
      {...(disabled !== undefined ? { disabled } : {})}
      {...(name ? { name } : {})}
      {...(defaultValue !== undefined ? { defaultValue } : {})}
      {...(required !== undefined ? { required } : {})}
    >
      <RSelect.Trigger className={cn("gg-select__trigger", className)} aria-label={ariaLabel}>
        <RSelect.Value placeholder={placeholder} />
        <RSelect.Icon className="gg-select__icon">
          <ChevronDown size={15} aria-hidden />
        </RSelect.Icon>
      </RSelect.Trigger>

      <RSelect.Portal>
        <RSelect.Content className="gg-select__content" position="popper" sideOffset={6}>
          <RSelect.ScrollUpButton className="gg-select__scroll">
            <ChevronUp size={14} aria-hidden />
          </RSelect.ScrollUpButton>
          <RSelect.Viewport className="gg-select__viewport">
            {options.map((o) => (
              <RSelect.Item
                key={o.value}
                value={o.value}
                {...(o.disabled !== undefined ? { disabled: o.disabled } : {})}
                className="gg-select__item"
              >
                <span className="gg-select__indicator">
                  <RSelect.ItemIndicator>
                    <Check size={14} aria-hidden />
                  </RSelect.ItemIndicator>
                </span>
                <span className="gg-select__labels">
                  <RSelect.ItemText>{o.label}</RSelect.ItemText>
                  {o.hint ? <span className="gg-select__hint">{o.hint}</span> : null}
                </span>
              </RSelect.Item>
            ))}
          </RSelect.Viewport>
          <RSelect.ScrollDownButton className="gg-select__scroll">
            <ChevronDown size={14} aria-hidden />
          </RSelect.ScrollDownButton>
        </RSelect.Content>
      </RSelect.Portal>
    </RSelect.Root>
  );
}

// -------------------------------------------------------------- Checkbox

export const Checkbox = forwardRef<
  HTMLButtonElement,
  {
    checked?: boolean | "indeterminate";
    onCheckedChange?: (c: boolean | "indeterminate") => void;
    id?: string;
    label?: ReactNode;
    hint?: string;
    disabled?: boolean;
    defaultChecked?: boolean;
    name?: string;
    value?: string;
  }
>(function Checkbox({ checked, onCheckedChange, id, label, hint, disabled, defaultChecked, name, value }, ref) {
  return (
    <div className="gg-choice">
      <RCheckbox.Root
        ref={ref}
        {...(id !== undefined ? { id } : {})}
        {...(checked !== undefined ? { checked } : {})}
        {...(onCheckedChange ? { onCheckedChange } : {})}
        {...(disabled !== undefined ? { disabled } : {})}
        {...(defaultChecked !== undefined ? { defaultChecked } : {})}
        {...(name ? { name } : {})}
        {...(value ? { value } : {})}
        className="gg-checkbox"
      >
        <RCheckbox.Indicator className="gg-checkbox__indicator">
          {checked === "indeterminate" ? <Minus size={12} aria-hidden /> : <Check size={12} aria-hidden />}
        </RCheckbox.Indicator>
      </RCheckbox.Root>
      {label ? (
        <RLabel.Root {...(id ? { htmlFor: id } : {})} className="gg-choice__label">
          {label}
          {hint ? <span className="gg-choice__hint">{hint}</span> : null}
        </RLabel.Root>
      ) : null}
    </div>
  );
});

// ----------------------------------------------------------- RadioGroup

export function RadioGroup({
  value,
  onValueChange,
  options,
  ariaLabel,
  orientation = "vertical",
}: {
  value?: string;
  onValueChange?: (v: string) => void;
  options: readonly SelectOption[];
  ariaLabel?: string;
  orientation?: "vertical" | "horizontal";
}) {
  return (
    <RRadio.Root
      {...(value !== undefined ? { value } : {})}
      {...(onValueChange ? { onValueChange } : {})}
      {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
      className={cn("gg-radiogroup", orientation === "horizontal" && "gg-radiogroup--horizontal")}
    >
      {options.map((o) => (
        <div key={o.value} className="gg-choice">
          <RRadio.Item
            value={o.value}
            id={`r-${o.value}`}
            {...(o.disabled !== undefined ? { disabled: o.disabled } : {})}
            className="gg-radio"
          >
            <RRadio.Indicator className="gg-radio__indicator" />
          </RRadio.Item>
          <RLabel.Root htmlFor={`r-${o.value}`} className="gg-choice__label">
            {o.label}
            {o.hint ? <span className="gg-choice__hint">{o.hint}</span> : null}
          </RLabel.Root>
        </div>
      ))}
    </RRadio.Root>
  );
}

// ---------------------------------------------------------- Date and time

/** A token-native date/time field. Browser datetime popovers are operating-
 * system UI and cannot follow the selected GhanaGeo material or brand hue. */
export function DateTimeInput({
  name,
  defaultValue = "",
  required,
  disabled,
  ariaLabel = "Date and time",
}: {
  name: string;
  defaultValue?: string;
  required?: boolean;
  disabled?: boolean;
  ariaLabel?: string;
}) {
  const [initialDate = "", initialTime = ""] = defaultValue.split("T");
  const [date, setDate] = useState(initialDate);
  const [time, setTime] = useState(initialTime.slice(0, 5));
  const value = useMemo(
    () => /^\d{4}-\d{2}-\d{2}$/.test(date) && /^\d{2}:\d{2}$/.test(time) ? `${date}T${time}` : "",
    [date, time],
  );
  return (
    <div className="gg-datetime" role="group" aria-label={ariaLabel}>
      <label className="gg-datetime__part">
        <CalendarDays size={15} aria-hidden />
        <span className="sr-only">Date</span>
        <input
          className="gg-datetime__input"
          value={date}
          onChange={(event) => setDate(event.target.value)}
          placeholder="YYYY-MM-DD"
          inputMode="numeric"
          pattern="\d{4}-\d{2}-\d{2}"
          disabled={disabled}
          aria-label={`${ariaLabel} date`}
        />
      </label>
      <label className="gg-datetime__part">
        <Clock3 size={15} aria-hidden />
        <span className="sr-only">Time</span>
        <input
          className="gg-datetime__input"
          value={time}
          onChange={(event) => setTime(event.target.value)}
          placeholder="HH:MM"
          inputMode="numeric"
          pattern="([01]\d|2[0-3]):[0-5]\d"
          disabled={disabled}
          aria-label={`${ariaLabel} time`}
        />
      </label>
      <input type="hidden" name={name} value={value} required={required} disabled={disabled} />
    </div>
  );
}

// --------------------------------------------------------------- Switch

export function Switch({
  checked,
  onCheckedChange,
  id,
  label,
  hint,
  disabled,
}: {
  checked?: boolean;
  onCheckedChange?: (c: boolean) => void;
  id?: string;
  label?: ReactNode;
  hint?: string;
  disabled?: boolean;
}) {
  return (
    <div className="gg-choice">
      <RSwitch.Root
        {...(id !== undefined ? { id } : {})}
        {...(checked !== undefined ? { checked } : {})}
        {...(onCheckedChange ? { onCheckedChange } : {})}
        {...(disabled !== undefined ? { disabled } : {})}
        className="gg-switch"
      >
        <RSwitch.Thumb className="gg-switch__thumb" />
      </RSwitch.Root>
      {label ? (
        <RLabel.Root {...(id ? { htmlFor: id } : {})} className="gg-choice__label">
          {label}
          {hint ? <span className="gg-choice__hint">{hint}</span> : null}
        </RLabel.Root>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------- Field

/** A labelled field wrapper, so every control gets a real <label> association
 *  rather than a nearby <span> that a screen reader cannot connect. */
export function Field({
  label,
  hint,
  error,
  htmlFor,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  htmlFor?: string;
  children: ReactNode;
}) {
  return (
    <div className="gg-field">
      <RLabel.Root {...(htmlFor ? { htmlFor } : {})} className="gg-field__label">
        {label}
      </RLabel.Root>
      {children}
      {error ? (
        <p className="gg-field__error" role="alert">{error}</p>
      ) : hint ? (
        <p className="gg-field__hint">{hint}</p>
      ) : null}
    </div>
  );
}
