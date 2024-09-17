const File = require('#models/file');
const Parameter = require('#models/parameters');
const Log = require('#models/logs');
const User = require('#models/user');  // Optional

// A file can have many parameters
File.hasMany(Parameter, { foreignKey: 'file_id' });
Parameter.belongsTo(File, { foreignKey: 'file_id' });

// A file can have many logs
File.hasMany(Log, { foreignKey: 'file_id' });
Log.belongsTo(File, { foreignKey: 'file_id' });

// A parameter can have many logs
Parameter.hasMany(Log, { foreignKey: 'parameter_id' });
Log.belongsTo(Parameter, { foreignKey: 'parameter_id' });

// (Optional) A user can have many files or parameters (depends on your implementation)
// User.hasMany(File, { foreignKey: 'user_id' });
// File.belongsTo(User, { foreignKey: 'user_id' });
