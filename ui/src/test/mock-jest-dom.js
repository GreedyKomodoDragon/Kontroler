// Mock replacement for @testing-library/jest-dom when running Vitest in this environment
// Provides no-op exports so importing packages that expect jest-dom won't fail.
export const toBeInTheDocument = () => {};
export default {};
