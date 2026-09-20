import { useId, useState } from "react";

type Props = {
  initialValue?: string;
  onSearch: (query: string) => void;
  busy?: boolean;
};

export default function SearchBox({ initialValue = "", onSearch, busy }: Props) {
  const inputId = useId();
  const [value, setValue] = useState(initialValue);

  return (
    <form
      role="search"
      onSubmit={(event) => {
        event.preventDefault();
        onSearch(value.trim());
      }}
      className="flex flex-col gap-2 sm:flex-row"
    >
      <div className="flex-1">
        <label htmlFor={inputId} className="block text-sm font-medium">
          Schule oder Kita suchen
        </label>
        <input
          id={inputId}
          type="search"
          name="q"
          value={value}
          autoComplete="off"
          onChange={(event) => setValue(event.target.value)}
          placeholder="z. B. Grundschule Stenn"
          className="mt-1 w-full rounded border border-line bg-white px-3 py-2"
        />
      </div>
      <button
        type="submit"
        className="self-end rounded bg-accent px-4 py-2 font-medium text-white disabled:opacity-60"
        disabled={busy}
      >
        Suchen
      </button>
    </form>
  );
}
