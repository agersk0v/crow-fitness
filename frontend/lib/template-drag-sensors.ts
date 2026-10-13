import { MouseSensor, TouchSensor } from "@dnd-kit/core";

function isInteractive(target: EventTarget | null) {
  return (
    target instanceof Element &&
    !!target.closest(
      'input, select, textarea, button, a, label, [contenteditable], [role="button"], [role="link"]',
    )
  );
}

export class CardMouseSensor extends MouseSensor {
  static activators: typeof MouseSensor.activators = MouseSensor.activators.map(
    (activator) => ({
      ...activator,
      handler: (event, options) =>
        !isInteractive(event.target) && activator.handler(event, options),
    }),
  );
}

export class CardTouchSensor extends TouchSensor {
  static activators: typeof TouchSensor.activators = TouchSensor.activators.map(
    (activator) => ({
      ...activator,
      handler: (event, options) =>
        !isInteractive(event.target) && activator.handler(event, options),
    }),
  );
}
