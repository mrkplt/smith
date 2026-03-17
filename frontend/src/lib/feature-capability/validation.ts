export type FeatureCapabilityInputs = {
  taskName: string;
  targetEnvironment: string;
};

export type ValidationErrors = Partial<Record<keyof FeatureCapabilityInputs, string>>;

/** Returns field-level validation errors for required feature capability inputs. */
export function validateFeatureCapabilityInputs(inputs: FeatureCapabilityInputs): ValidationErrors {
  const errors: ValidationErrors = {};

  if (inputs.taskName.trim().length === 0) {
    errors.taskName = 'Task name is required.';
  }

  if (inputs.targetEnvironment.trim().length === 0) {
    errors.targetEnvironment = 'Target environment is required.';
  }

  return errors;
}
