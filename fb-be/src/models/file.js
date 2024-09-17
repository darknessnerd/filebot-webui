const { DataTypes } = require('sequelize');
const sequelize = require('#configs/db');  // Import the initialized Sequelize instance

const File = sequelize.define('File', {
    file_id: {
        type: DataTypes.INTEGER,
        primaryKey: true,
        autoIncrement: true,
    },
    file_name: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    file_path: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    file_size: {
        type: DataTypes.BIGINT,
    },
    file_type: {
        type: DataTypes.STRING,
    },
    created_at: {
        type: DataTypes.DATE,
        defaultValue: DataTypes.NOW,
    },
}, {
    tableName: 'files',
    timestamps: false,  // Disable the automatic 'createdAt' and 'updatedAt' fields
});

module.exports = File;
