import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SeasonSelector } from './SeasonSelector';

describe('<SeasonSelector />', () => {
  it('renders an "All" chip plus one chip per available season', () => {
    render(<SeasonSelector available={[1, 2, 3]} selected={null} onChange={() => undefined} />);
    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'S1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'S2' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'S3' })).toBeInTheDocument();
  });

  it('marks only the "All" chip pressed when selected is null (season chips render unpressed)', () => {
    render(<SeasonSelector available={[1, 2, 3]} selected={null} onChange={() => undefined} />);
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'S1' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'S2' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'S3' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('marks only the selected season chips pressed when selected is an explicit list', () => {
    render(<SeasonSelector available={[1, 2, 3]} selected={[1, 3]} onChange={() => undefined} />);
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'S1' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'S2' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'S3' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('clicking a chip while in "all" mode selects only that season', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={null} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S2' }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([2]);
  });

  it('toggling off the last selected chip emits null (collapses back to all-seasons)', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={[2]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S2' }));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it('toggling "All" while in explicit-list mode emits null', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={[1]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it('toggling "All" when already in all-mode is a no-op', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={null} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('re-selecting the last missing season normalizes back to null', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={[1, 3]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S2' }));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it('adding a season into an explicit list keeps the list ascending-sorted', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3, 4]} selected={[3]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S1' }));
    expect(onChange).toHaveBeenCalledWith([1, 3]);
  });

  it('renders nothing when only one season is available', () => {
    const { container } = render(
      <SeasonSelector available={[1]} selected={null} onChange={() => undefined} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when no seasons are available', () => {
    const { container } = render(
      <SeasonSelector available={[]} selected={null} onChange={() => undefined} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('toggling off the only real chip when selected has stale entries emits null', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={[1, 99]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S1' }));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it('toggling on a chip when selected contains stale seasons does not collapse to all', () => {
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={[1, 99]} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'S2' }));
    expect(onChange).toHaveBeenCalledWith([1, 2]);
  });

  it('keyboard activation via Enter and Space triggers onChange (native <button>)', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SeasonSelector available={[1, 2, 3]} selected={null} onChange={onChange} />);
    const s2 = screen.getByRole('button', { name: 'S2' });
    s2.focus();
    await user.keyboard('{Enter}');
    expect(onChange).toHaveBeenLastCalledWith([2]);

    onChange.mockClear();
    s2.focus();
    await user.keyboard(' ');
    expect(onChange).toHaveBeenLastCalledWith([2]);
  });
});
