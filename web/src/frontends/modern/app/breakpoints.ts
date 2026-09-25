export const breakpoints = {
  narrow: 420,
  mobile: 760,
  compact: 1150,
  wide: 1700,
} as const

export const desktopMediaQuery = `(width > ${breakpoints.mobile}px)`
